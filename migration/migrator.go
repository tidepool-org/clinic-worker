package migration

import (
	"context"
	"fmt"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	threadiness                      = 4
	patientMigrationTimeout          = 10 * time.Second
	postMigrationRole                = "clinician"
	patientMigrationTemplate         = "migrate_patient"
	clinicMigrationCompletedTemplate = "clinic_migration_complete"
	migrationStatusRunning           = "RUNNING"
	migrationStatusCompleted         = "COMPLETED"
)

type Migrator interface {
	MigratePatients(ctx context.Context, userId, clinicId string) error
}

type migrator struct {
	clinics     clinics.ClientWithResponsesInterface
	gatekeeper  clients.Gatekeeper
	logger      *zap.SugaredLogger
	rateLimiter *RateLimiter
	mailer      clients.MailerClient
	seagull     clients.Seagull
	shoreline   shoreline.Client
}

var _ Migrator = &migrator{}

type MigratorParams struct {
	fx.In

	Clinics     clinics.ClientWithResponsesInterface
	Gatekeeper  clients.Gatekeeper
	Logger      *zap.SugaredLogger
	RateLimiter *RateLimiter
	Mailer      clients.MailerClient
	Seagull     clients.Seagull
	Shoreline   shoreline.Client
}

func NewMigrator(p MigratorParams) (Migrator, error) {
	return &migrator{
		clinics:     p.Clinics,
		gatekeeper:  p.Gatekeeper,
		logger:      p.Logger,
		mailer:      p.Mailer,
		rateLimiter: p.RateLimiter,
		seagull:     p.Seagull,
		shoreline:   p.Shoreline,
	}, nil
}

func (m *migrator) MigratePatients(ctx context.Context, userId, clinicId string) error {
	m.logger.Infof("Starting migration of patients of legacy clinician user %v to clinic %v", userId, clinicId)
	migration, err := m.createMigration(ctx, userId, clinicId)
	if err != nil {
		return err
	}

	m.logger.Infof("Updating migration status of user %v to running", userId)
	if err := m.updateMigrationsStatus(ctx, migration, migrationStatusRunning); err != nil {
		return err
	}

	// Make sure the clinician cannot use legacy version of uploader
	m.logger.Infof("Removing legacy clinician role of user %v", userId)
	if err := m.removeLegacyClinicianRole(userId); err != nil {
		return err
	}

	// Make sure the clinician cannot create legacy custodial accounts in uploader
	m.logger.Infof("Deleting active user sessions of user %v", userId)
	if err := m.shoreline.DeleteUserSessions(userId, m.shoreline.TokenProvide()); err != nil {
		return err
	}

	sem := semaphore.NewWeighted(threadiness)
	eg, c := errgroup.WithContext(ctx)

	m.logger.Infof("Migrating %v patients from legacy clinician user %v to %v", len(migration.legacyPatients), userId, clinicId)
	// the entire loop must be launched in an errgroup goroutine
	// to make sure context cancellations are handled correctly
	eg.Go(func() error {
		for patientId, perms := range migration.legacyPatients {
			if c.Err() != nil {
				return err
			}
			if err := sem.Acquire(context.TODO(), 1); err != nil {
				m.logger.Errorw("Failed to acquire semaphore", zap.Error(err))
				return err
			}

			// we can't pass arguments to errgroup goroutines
			// we need to explicitly redefine the variables,
			// because we're launching the goroutines in a loop
			perms := perms
			patientId := patientId
			eg.Go(func() error {
				defer sem.Release(1)

				// blocks if the rate limit is exceeded
				m.rateLimiter.WaitOrContinue()

				mCtx, cancel := context.WithTimeout(ctx, patientMigrationTimeout)
				defer cancel() // free up resources if migrations finishes before the timeout is exceeded

				return m.migratePatient(mCtx, migration, patientId, perms)
			})
		}

		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	} else {
		m.logger.Infof("Patients of clinician user %v were successfully migrated to clinic %v", userId, clinicId)
	}

	m.logger.Infof("Updating migration status of user %v to completed", userId)
	if err := m.updateMigrationsStatus(ctx, migration, migrationStatusCompleted); err != nil {
		return err
	}

	if err := m.sendMigrationCompletedEmail(ctx, migration); err != nil {
		m.logger.Errorf("error sending migration completed email for clinic %v: %w", clinicId, err)
		return err
	}

	return nil
}

func (m *migrator) createMigration(ctx context.Context, userId, clinicId string) (*Migration, error) {
	response, err := m.clinics.GetClinicWithResponse(ctx, clinics.ClinicId(clinicId))
	if err != nil {
		return nil, err
	} else if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response when fetching clinic %s", clinicId)
	}

	patients, err := m.gatekeeper.GroupsForUser(userId)
	if err != nil {
		return nil, err
	}

	// The owner of the account is not a patient of a clinic
	delete(patients, userId)

	profile := &LegacyClinicianProfile{}
	err = m.seagull.GetCollection(userId, "profile", m.shoreline.TokenProvide(), profile)
	if err != nil {
		return nil, err
	}

	return &Migration{
		legacyClinicianUserId:  userId,
		legacyClinicianProfile: profile,
		clinic:                 response.JSON200,
		legacyPatients:         patients,
	}, nil
}

func (m *migrator) updateMigrationsStatus(ctx context.Context, migration *Migration, status string) error {
	body := clinics.UpdateMigrationJSONRequestBody{
		Status: clinics.MigrationStatus(status),
	}
	resp, err := m.clinics.UpdateMigrationWithResponse(ctx, migration.clinic.Id, clinics.UserId(migration.legacyClinicianUserId), body)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected response code when updating migration status: %v", resp.StatusCode())
	}
	return nil
}

func (m *migrator) removeLegacyClinicianRole(userId string) error {
	roles := []string{postMigrationRole}
	update := shoreline.UserUpdate{
		Roles: &roles,
	}
	return m.shoreline.UpdateUser(userId, update, m.shoreline.TokenProvide())
}

func (m *migrator) migratePatient(ctx context.Context, migration *Migration, patientId string, permissions clients.Permissions) error {
	patient, err := m.createPatient(ctx, migration, patientId, permissions)
	if err != nil {
		return err
	}
	if patient == nil {
		m.logger.Infow(
			"Patient couldn't be migrated, because the user doesn't exist anymore",
			"clinicId", string(migration.clinic.Id), "userId", patientId,
		)
		return nil
	}
	if err = m.sendMigrationEmail(ctx, migration, patient); err != nil {
		return err
	}
	if err = m.removeSharingConnection(migration.legacyClinicianUserId, patientId); err != nil {
		return err
	}
	return nil
}

func (m *migrator) createPatient(ctx context.Context, migration *Migration, patientId string, permissions clients.Permissions) (*clinics.Patient, error) {
	clinicId := string(migration.clinic.Id)
	m.logger.Infof("Migrating patient %v to clinic %v", patientId, clinicId)
	var patient *clinics.Patient
	var err error

	isMigrated := true
	legacyClinicianId := clinics.TidepoolUserId(migration.legacyClinicianUserId)
	body := clinics.CreatePatientFromUserJSONRequestBody{
		IsMigrated:        &isMigrated,
		LegacyClinicianId: &legacyClinicianId,
		Permissions:       mapPermissions(permissions),
	}
	response, err := m.clinics.CreatePatientFromUserWithResponse(
		ctx,
		clinics.ClinicId(clinicId),
		clinics.PatientId(patientId),
		body,
	)
	if err == nil {
		if response.StatusCode() == http.StatusOK {
			// the patient was successfully migrated
			patient = response.JSON200
		} else if response.StatusCode() == http.StatusConflict {
			// the user is already a patient of the clinic
			var patientResponse *clinics.GetPatientResponse
			patientResponse, err = m.clinics.GetPatientWithResponse(ctx, clinics.ClinicId(clinicId), clinics.PatientId(patientId))
			if err != nil {
				err = fmt.Errorf("error fetching patient: %v", err)
			} else if patientResponse.StatusCode() == http.StatusOK {
				patient = patientResponse.JSON200
			} else {
				err = fmt.Errorf("unexpected status code when fetching patient %v", patientResponse.StatusCode())
			}
		} else if response.StatusCode() == http.StatusNotFound {
			return nil, nil
		} else {
			err = fmt.Errorf("unexpected status code %v", response.StatusCode())
		}
	}

	if err != nil {
		m.logger.Errorw(fmt.Sprintf("error occurred while migrating patient %v to clinic %v", patientId, clinicId), zap.Error(err))
	}

	return patient, err
}

func (m *migrator) removeSharingConnection(userId, patientId string) error {
	m.logger.Infof("Removing sharing connection between legacy clinician %v and patient %v", userId, patientId)
	_, err := m.gatekeeper.SetPermissions(userId, patientId, nil)
	return err
}

func (m *migrator) sendMigrationEmail(ctx context.Context, migrationContext *Migration, patient *clinics.Patient) error {
	if patient.Email == nil || *patient.Email == "" {
		m.logger.Infof("Skipping sending of migration email to user %s because email address is empty", string(patient.Id))
		return nil
	}

	m.logger.Infof("Sending migration email to user %s", string(patient.Id))
	email := events.SendEmailTemplateEvent{
		Recipient: *patient.Email,
		Template:  patientMigrationTemplate,
		Variables: map[string]string{
			"LegacyClinicianName": migrationContext.legacyClinicianProfile.Name,
			"ClinicName":          migrationContext.clinic.Name,
		},
	}

	return m.mailer.SendEmailTemplate(ctx, email)
}

func (m *migrator) sendMigrationCompletedEmail(ctx context.Context, migrationContext *Migration) error {
	m.logger.Debugf("Fetching legacy clinician user %v", migrationContext.legacyClinicianUserId)
	user, err := m.shoreline.GetUser(migrationContext.legacyClinicianUserId, m.shoreline.TokenProvide())
	if err != nil {
		return err
	}

	m.logger.Infof("Sending migration completed email to user %s", migrationContext.legacyClinicianUserId)
	email := events.SendEmailTemplateEvent{
		Recipient: user.Username,
		Template:  clinicMigrationCompletedTemplate,
		Variables: map[string]string{
			"ClinicName": migrationContext.clinic.Name,
		},
	}

	return m.mailer.SendEmailTemplate(ctx, email)
}

func mapPermissions(permissions clients.Permissions) *clinics.PatientPermissions {
	mapped := clinics.PatientPermissions{}
	permission := make(map[string]interface{})
	if _, ok := permissions["custodian"]; ok {
		mapped.Custodian = &permission
	}
	if _, ok := permissions["note"]; ok {
		mapped.Note = &permission
	}
	if _, ok := permissions["view"]; ok {
		mapped.View = &permission
	}
	if _, ok := permissions["upload"]; ok {
		mapped.Upload = &permission
	}
	return &mapped
}
