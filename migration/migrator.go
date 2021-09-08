package migration

import (
	"context"
	"fmt"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"go.uber.org/zap"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	threadiness             = 4
	patientMigrationTimeout = 10 * time.Second
	postMigrationRole       = "migrated_clinic"
)

type Migrator interface {
	MigratePatients(ctx context.Context, userId, clinicId string) error
}

type migrator struct {
	logger     *zap.SugaredLogger
	gatekeeper clients.Gatekeeper
	shoreline  shoreline.Client
	clinics    clinics.ClientWithResponsesInterface
}

var _ Migrator = &migrator{}

func NewMigrator(logger *zap.SugaredLogger, gatekeeper clients.Gatekeeper, shoreline shoreline.Client, clinics clinics.ClientWithResponsesInterface) (Migrator, error) {
	return &migrator{
		logger:     logger,
		shoreline:  shoreline,
		gatekeeper: gatekeeper,
		clinics:    clinics,
	}, nil
}

func (m *migrator) MigratePatients(ctx context.Context, userId, clinicId string) error {
	m.logger.Infof("Starting migration of patients of legacy clinician user %v to clinic %v", userId, clinicId)
	permissions, err := m.gatekeeper.GroupsForUser(userId)
	if err != nil {
		return err
	}

	// The owner of the account is not a patient of a clinic
	delete(permissions, userId)

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

	m.logger.Infof("Migrating %v patients from legacy clinician user %v to %v", len(permissions), userId, clinicId)
	for patientId, perms := range permissions {
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
			return m.migratePatient(mCtx, userId, clinicId, patientId, perms)
		})
	}

	return eg.Wait()
}

func (m *migrator) removeLegacyClinicianRole(userId string) error {
	roles := []string{postMigrationRole}
	update := shoreline.UserUpdate{
		Roles: &roles,
	}
	return m.shoreline.UpdateUser(userId, update, m.shoreline.TokenProvide())
}

func (m *migrator) migratePatient(ctx context.Context, userId, clinicId, patientId string, permissions clients.Permissions) error {
	if err := m.createPatient(ctx, clinicId, patientId, permissions); err != nil {
		return err
	}
	if err := m.removeSharingConnection(userId, patientId); err != nil {
		return err
	}
	return nil
}

func (m *migrator) createPatient(ctx context.Context, clinicId, patientId string, permissions clients.Permissions) error {
	m.logger.Infof("Migrating patient %v to clinic %v", patientId, clinicId)
	isMigrated := true
	body := clinics.CreatePatientFromUserJSONRequestBody{
		IsMigrated : &isMigrated,
		Permissions: mapPermissions(permissions),
	}
	response, err := m.clinics.CreatePatientFromUserWithResponse(
		ctx,
		clinics.ClinicId(clinicId),
		clinics.PatientId(patientId),
		body,
	)
	if err == nil && response.StatusCode() != http.StatusConflict && response.StatusCode() != http.StatusOK {
		err = fmt.Errorf("unexpected status code %v", response.StatusCode())
	}
	if err != nil {
		m.logger.Errorw(fmt.Sprintf("error occurred while migrating patient %v to clinic %v", patientId, clinicId), zap.Error(err))
		return err
	}
	return err
}

func (m *migrator) removeSharingConnection(userId, patientId string) error {
	m.logger.Infof("Removing sharing connection between legacy clinician %v and patient %v", userId, patientId)
	_, err := m.gatekeeper.SetPermissions(userId, patientId, nil)
	return err
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
