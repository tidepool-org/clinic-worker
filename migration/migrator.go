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
	threadiness              = 4
	patientMigrationTimeout  = 10 * time.Second
	postMigrationRole        = "migrated_clinic"
	patientMigrationTemplate = "migrate_patient"
)

type Migrator interface {
	MigratePatients(ctx context.Context, userId, clinicId string) error
}

type migrator struct {
	clinics    clinics.ClientWithResponsesInterface
	gatekeeper clients.Gatekeeper
	logger     *zap.SugaredLogger
	mailer     *clients.MailerClient
	seagull    clients.Seagull
	shoreline  shoreline.Client
}

var _ Migrator = &migrator{}

type MigratorParams struct {
	fx.In

	Clinics    clinics.ClientWithResponsesInterface
	Gatekeeper clients.Gatekeeper
	Logger     *zap.SugaredLogger
	Mailer     *clients.MailerClient
	Seagull    clients.Seagull
	Shoreline  shoreline.Client
}

func NewMigrator(p MigratorParams) (Migrator, error) {
	return &migrator{
		clinics:    p.Clinics,
		gatekeeper: p.Gatekeeper,
		logger:     p.Logger,
		mailer:     p.Mailer,
		seagull:    p.Seagull,
		shoreline:  p.Shoreline,
	}, nil
}

func (m *migrator) MigratePatients(ctx context.Context, userId, clinicId string) error {
	m.logger.Infof("Starting migration of patients of legacy clinician user %v to clinic %v", userId, clinicId)
	migration, err := m.createMigration(ctx, userId, clinicId)
	if err != nil {
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
	for patientId, perms := range migration.legacyPatients {
		if c.Err() != nil {
			break
		}
		if err := sem.Acquire(context.TODO(), 1); err != nil {
			m.logger.Errorw("Failed to acquire semaphore", zap.Error(err))
			break
		}

		// we can't pass arguments to errgroup goroutines
		// we need to explicitly redefine the variables,
		// because we're launching the goroutines in a loop
		perms := perms
		patientId := patientId
		eg.Go(func() error {
			defer sem.Release(1)
			mCtx, _ := context.WithTimeout(ctx, patientMigrationTimeout)
			return m.migratePatient(mCtx, migration, patientId, perms)
		})
	}

	err = eg.Wait()
	if err == nil {
		m.logger.Infof("Legacy clinician user %v was successfully migrated to clinic %v", userId, clinicId)
	}
	return err
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
	body := clinics.CreatePatientFromUserJSONRequestBody{
		IsMigrated:  &isMigrated,
		Permissions: mapPermissions(permissions),
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
			if err != nil{
				err = fmt.Errorf("error fetching patient: %v", err)
			} else if patientResponse.StatusCode() == http.StatusOK {
				patient = patientResponse.JSON200
			} else {
				err = fmt.Errorf("unexpected status code when fetching patient %v", response.StatusCode())
			}
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
