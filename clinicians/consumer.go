package clinicians

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/marketo"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	cliniciansTopic      = "clinic.clinicians"
	defaultClinicianName = "Clinic administrator"
	defaultTimeout       = 30 * time.Second
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateConsumerGroup,
})

type ClinicianCDCConsumer struct {
	logger *zap.SugaredLogger

	clinics       clinics.ClientWithResponsesInterface
	mailer        clients.MailerClient
	marketoClient marketo.Client
	shoreline     shoreline.Client
}

type Params struct {
	fx.In

	Clinics       clinics.ClientWithResponsesInterface
	Logger        *zap.SugaredLogger
	Mailer        clients.MailerClient
	MarketoClient marketo.Client
	Shoreline     shoreline.Client
}

func CreateConsumerGroup(p Params) (events.EventConsumer, error) {
	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = cliniciansTopic

	return events.NewFaultTolerantConsumerGroup(config, CreateConsumer(p))
}

func CreateConsumer(p Params) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		delegate, err := NewClinicianCDCConsumer(p)
		if err != nil {
			return nil, err
		}
		return cdc.NewRetryingConsumer(delegate), nil
	}
}

func NewClinicianCDCConsumer(p Params) (events.MessageConsumer, error) {
	return &ClinicianCDCConsumer{
		clinics:       p.Clinics,
		logger:        p.Logger,
		mailer:        p.Mailer,
		marketoClient: p.MarketoClient,
		shoreline:     p.Shoreline,
	}, nil
}

func (p *ClinicianCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (p *ClinicianCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	return p.handleMessage(cm)
}

func (p *ClinicianCDCConsumer) handleMessage(cm *sarama.ConsumerMessage) error {
	p.logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := PatientCDCEvent{
		Offset: cm.Offset,
	}
	if err := p.unmarshalEvent(cm.Value, &event); err != nil {
		p.logger.Warnw("unable to unmarshal message", "offset", cm.Offset, zap.Error(err))
		return err
	}

	if err := p.handleCDCEvent(event); err != nil {
		p.logger.Errorw("unable to process cdc event", "offset", cm.Offset, zap.Error(err))
		return err
	}
	return nil
}

func (p *ClinicianCDCConsumer) unmarshalEvent(value []byte, event *PatientCDCEvent) error {
	message, err := strconv.Unquote(string(value))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(message), event)
}

func (p *ClinicianCDCConsumer) handleCDCEvent(event PatientCDCEvent) error {
	if !event.ShouldApplyUpdates() {
		p.logger.Debugw("skipping handling of event", "offset", event.Offset)
		return nil
	}

	p.logger.Infow("processing event", "event", event)

	// First so a failure later in the handler retries an idempotent operation: the clinic
	// service applies the profile with a partial update keyed by userId.
	if event.ShouldBackfillSecurityProfile() {
		if err := p.backfillSecurityProfile(event.FullDocument.UserId); err != nil {
			return err
		}
	}

	if event.FullDocument.UserId != "" {
		if err := p.refreshMarketoUserDetails(event.FullDocument.UserId); err != nil {
			return err
		}
	}

	if err := p.sendPermissionsUpdatedEmail(event); err != nil {
		return err
	}

	return nil
}

// backfillSecurityProfile seeds the clinician's security profile from the user's current
// security posture as reported by shoreline (MFA state and IdP links computed live from
// keycloak, last login from the user's last_login_time attribute). The clinician CDC stream
// marks the join moment; without this, the profile would stay empty until the user's next
// login, MFA, or IdP transition flows through the keycloak user-activity pipeline.
func (p *ClinicianCDCConsumer) backfillSecurityProfile(userId string) error {
	user, err := p.shoreline.GetUser(userId, p.shoreline.TokenProvide())
	if err != nil {
		var statusErr *status.StatusError
		if errors.As(err, &statusErr) && statusErr.Code == http.StatusNotFound {
			// The user was deleted before this event was processed.
			p.logger.Infow("skipping security profile backfill for deleted user", "userId", userId)
			return nil
		}
		return fmt.Errorf("unable to get user %v: %w", userId, err)
	}
	if user == nil || user.SecurityProfile == nil {
		// Unmigrated user, or shoreline doesn't report security profiles yet.
		p.logger.Infow("no security profile reported for user", "userId", userId)
		return nil
	}

	profile := user.SecurityProfile
	identityProviders := make([]clinics.ClinicianIdentityProviderV1, 0, len(profile.IdentityProviders))
	for _, idp := range profile.IdentityProviders {
		identityProviders = append(identityProviders, clinics.ClinicianIdentityProviderV1{
			Alias: idp.Alias,
			Name:  idp.Name,
		})
	}
	update := clinics.UpdateClinicianSecurityProfileJSONRequestBody{
		MfaEnabled:        &profile.MfaEnabled,
		IdentityProviders: &identityProviders,
	}
	if profile.LastLoginTime != nil {
		lastLogin := clinics.DatetimeV1(profile.LastLoginTime.Format(time.RFC3339))
		update.LastLoginTime = &lastLogin
	}

	p.logger.Infow("backfilling clinician security profile", "userId", userId)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	response, err := p.clinics.UpdateClinicianSecurityProfileWithResponse(ctx, clinics.UserId(userId), update)
	if err != nil {
		return err
	}
	if !(response.StatusCode() == http.StatusNoContent || response.StatusCode() == http.StatusNotFound) {
		return fmt.Errorf("unexpected status code when updating clinician security profile %v", response.StatusCode())
	}
	return nil
}

func (p *ClinicianCDCConsumer) sendPermissionsUpdatedEmail(event PatientCDCEvent) error {
	if count := len(event.UpdateDescription.UpdatedFields.RolesUpdates); count > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		clinicId := event.FullDocument.ClinicId.Value
		clinicianId := event.FullDocument.UserId

		update := event.UpdateDescription.UpdatedFields.RolesUpdates[count-1]
		updatedByUserId := update.UpdatedBy

		eg, egCtx := errgroup.WithContext(ctx)
		recipientChan := make(chan string, 1)
		clinicNameChan := make(chan string, 1)
		updatedByUserNameChan := make(chan string, 1)

		eg.Go(func() error {
			defer close(recipientChan)
			email, err := p.getUserEmail(clinicianId)
			if err != nil {
				return err
			}

			recipientChan <- email
			return nil
		})

		eg.Go(func() error {
			defer close(clinicNameChan)
			clinicName, err := p.getClinicName(egCtx, clinicId)
			if err != nil {
				return err
			}

			clinicNameChan <- clinicName
			return nil
		})

		eg.Go(func() error {
			defer close(updatedByUserNameChan)
			clinicianName, err := p.getClinicianName(egCtx, clinicId, updatedByUserId)
			if err != nil {
				return err
			}

			updatedByUserNameChan <- clinicianName
			return nil
		})

		if err := eg.Wait(); err != nil {
			return err
		}

		// The user was deleted before this event was processed
		recipient := <-recipientChan
		if recipient == "" {
			return nil
		}

		p.logger.Infow("Sending clinician permissions updated email",
			"userId", clinicianId,
			"email", recipient,
		)
		template := events.SendEmailTemplateEvent{
			Recipient: recipient,
			Template:  "clinician_permissions_updated",
			Variables: map[string]string{
				"ClinicName":    <-clinicNameChan,
				"ClinicianName": <-updatedByUserNameChan,
			},
		}

		return p.mailer.SendEmailTemplate(ctx, template)
	}

	return nil
}

func (p *ClinicianCDCConsumer) getClinicianName(ctx context.Context, clinicId, clinicianId string) (string, error) {
	p.logger.Debugw("Fetching clinician by id", "clinicId", clinicId, "clinicianId", clinicianId)
	response, err := p.clinics.GetClinicianWithResponse(ctx, clinics.ClinicId(clinicId), clinics.ClinicianId(clinicianId))
	if err != nil {
		return defaultClinicianName, err
	}

	if response.StatusCode() == http.StatusOK {
		if response.JSON200.Name != nil && len(*response.JSON200.Name) > 0 {
			return *response.JSON200.Name, nil
		}
	} else if response.StatusCode() != http.StatusNotFound {
		return defaultClinicianName, fmt.Errorf("unexpected status code when fetching clinician %v", response.StatusCode())
	}

	return defaultClinicianName, nil
}

func (p *ClinicianCDCConsumer) getClinicName(ctx context.Context, clinicId string) (string, error) {
	p.logger.Debugw("Fetching clinic by id", "clinicId", clinicId)
	response, err := p.clinics.GetClinicWithResponse(ctx, clinics.ClinicId(clinicId))
	if err != nil {
		return "", err
	}

	if response.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("unexpected status code when fetching clinic %v", response.StatusCode())
	}

	return response.JSON200.Name, nil
}

func (p *ClinicianCDCConsumer) getUserEmail(userId string) (string, error) {
	p.logger.Debugw("Fetching user by id", "userId", userId)
	user, err := p.shoreline.GetUser(userId, p.shoreline.TokenProvide())
	if err != nil {
		if e, ok := err.(*status.StatusError); ok && e.Code == http.StatusNotFound {
			// User was probably deleted, nothing we can do
			return "", nil
		}
		return "", fmt.Errorf("unexpected error when fetching user: %w", err)
	}
	return user.Username, nil
}

func (p *ClinicianCDCConsumer) refreshMarketoUserDetails(userId string) error {
	p.logger.Debugw("Refreshing marketo user details", "userId", userId)
	if err := p.marketoClient.RefreshUserDetails(userId); err != nil {
		// Log the error and move on to avoid getting the process stuck
		p.logger.Errorw("unable to refresh marketo user details", "userId", userId, zap.Error(err))
	}
	return nil
}
