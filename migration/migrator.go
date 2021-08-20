package migration

import (
	"context"
	"fmt"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
	"go.uber.org/zap"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	threadiness             = 4
	patientMigrationTimeout = 10 * time.Second
)

type Migrator interface {
	MigratePatients(ctx context.Context, userId, clinicId string) error
}

type migrator struct {
	logger     *zap.SugaredLogger
	gatekeeper clients.Gatekeeper
	clinics    clinics.ClientWithResponsesInterface
}

var _ Migrator = &migrator{}

func NewMigrator(logger *zap.SugaredLogger, gatekeeper clients.Gatekeeper, clinics clinics.ClientWithResponsesInterface) (Migrator, error) {
	return &migrator{
		logger:     logger,
		gatekeeper: gatekeeper,
		clinics:    clinics,
	}, nil
}

func (m *migrator) MigratePatients(ctx context.Context, userId, clinicId string) error {
	m.logger.Infof("Starting migration of patients of legacy clinician user %v to clinic %v", userId, clinicId)
	permissions, err := m.gatekeeper.UsersInGroup(userId)
	if err != nil {
		return err
	}

	m.logger.Infof("Migrating %v patients from legacy clinician user %v to %v", len(permissions), userId, clinicId)

	sem := semaphore.NewWeighted(threadiness)
	eg, c := errgroup.WithContext(ctx)

	for patientId, perms := range permissions {
		if c.Err() != nil {
			break
		}
		if err := sem.Acquire(context.TODO(), 1); err != nil {
			m.logger.Errorw("Failed to acquire semaphore", zap.Error(err))
			break
		}

		eg.Go(func() error {
			defer sem.Release(1)
			mCtx, _ := context.WithTimeout(ctx, patientMigrationTimeout)
			return m.migratePatient(mCtx, userId, clinicId, patientId, perms)
		})
	}

	return eg.Wait()
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
	body := clinics.CreatePatientFromUserJSONRequestBody{
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
	_, err := m.gatekeeper.SetPermissions(userId, patientId, make(clients.Permissions))
	return err
}

func mapPermissions(permissions clients.Permissions) *clinics.PatientPermissions {
	mapped := clinics.PatientPermissions{}
	permission := make(map[string]interface{})
	if _, ok := permissions["custodian"]; ok {
		mapped.Custodian = &permission
	}
	if _, ok := permissions["view"]; ok {
		mapped.Note = &permission
	}
	if _, ok := permissions["note"]; ok {
		mapped.View = &permission
	}
	if _, ok := permissions["upload"]; ok {
		mapped.Upload = &permission
	}
	return &mapped
}
