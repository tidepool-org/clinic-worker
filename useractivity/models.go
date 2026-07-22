package useractivity

import (
	"encoding/json"
	"fmt"
	"time"

	clinics "github.com/tidepool-org/clinic/client"
)

// Event types emitted by the keycloak-extensions user-activity outbox
// (org.tidepool.keycloak.extensions.activity.UserActivityEventEntity).
const (
	EventTypeLogin           = "LOGIN"
	EventTypeMfaEnabled      = "MFA_ENABLED"
	EventTypeMfaDisabled     = "MFA_DISABLED"
	EventTypeIdpLinksChanged = "IDP_LINKS_CHANGED"
)

// Debezium operation codes (op field of the change envelope).
const (
	opCreate = "c" // insert
	opRead   = "r" // snapshot read of an existing row
	opUpdate = "u" // update (not expected: the outbox is insert-only)
	opDelete = "d" // delete (cleanup/pruning — ignored)
)

// Envelope is the Debezium change event for a row of the keycloak Postgres outbox
// table tidepool_user_activity_event, using the standard JSON value format with
// schemas disabled (value.converter.schemas.enable=false):
//
//	{"op":"c","before":null,"after":{...row...},"ts_ms":...}
type Envelope struct {
	Offset int64  `json:"-"`
	Op     string `json:"op"`
	After  *Row   `json:"after"`
}

// Row mirrors the outbox table columns. Postgres folds the unquoted Liquibase
// identifiers to lower case, so the Debezium field names are snake_case.
type Row struct {
	ID        string `json:"id"`
	RealmID   string `json:"realm_id"`
	UserID    string `json:"user_id"`
	EventType string `json:"event_type"`
	// IdentityProviders is a JSON-array string (or null), set only on
	// IDP_LINKS_CHANGED rows. MFA enabled/disabled state is carried by the event
	// type alone (MFA_ENABLED / MFA_DISABLED); there is no boolean column.
	IdentityProviders *string `json:"identity_providers"`
	EventTime         int64   `json:"event_time"` // epoch milliseconds
}

// ShouldApplyUpdates reports whether the change event carries a user-activity row
// we project into the clinic. Deletes are cleanup only and are ignored; rows
// without a user id or with an unrecognized event type are skipped.
func (e Envelope) ShouldApplyUpdates() bool {
	if e.Op == opDelete || e.After == nil {
		return false
	}
	if e.Op != opCreate && e.Op != opRead && e.Op != opUpdate {
		return false
	}
	row := e.After
	if row.UserID == "" {
		return false
	}
	switch row.EventType {
	case EventTypeLogin, EventTypeMfaEnabled, EventTypeMfaDisabled, EventTypeIdpLinksChanged:
		return true
	default:
		return false
	}
}

// CreateUpdateBody maps the outbox row to a partial security-profile update. Only
// the fields relevant to the row's event type are set; everything else is left
// untouched by the clinic service's PATCH semantics.
func (e Envelope) CreateUpdateBody() (*clinics.ClinicianSecurityProfileUpdateV1, error) {
	row := e.After
	eventTime := clinics.DatetimeV1(time.UnixMilli(row.EventTime).UTC().Format(time.RFC3339))
	body := &clinics.ClinicianSecurityProfileUpdateV1{}

	switch row.EventType {
	case EventTypeLogin:
		body.LastLoginTime = &eventTime
	case EventTypeMfaEnabled:
		enabled := true
		body.MfaEnabled = &enabled
		body.MfaEnabledTime = &eventTime
	case EventTypeMfaDisabled:
		disabled := false
		body.MfaEnabled = &disabled
		// The clinic service clears mfaEnabledTime whenever MFA is disabled.
	case EventTypeIdpLinksChanged:
		providers, err := parseIdentityProviders(row.IdentityProviders)
		if err != nil {
			return nil, err
		}
		// Always set, so an empty array wholesale-clears the user's linked IdPs.
		body.IdentityProviders = &providers
	default:
		return nil, fmt.Errorf("unsupported event type %q", row.EventType)
	}

	return body, nil
}

// parseIdentityProviders decodes the IDENTITY_PROVIDERS JSON-array string into the
// clinic client type. A null/empty column yields an empty (non-nil) slice.
func parseIdentityProviders(raw *string) ([]clinics.ClinicianIdentityProviderV1, error) {
	providers := []clinics.ClinicianIdentityProviderV1{}
	if raw == nil || *raw == "" {
		return providers, nil
	}
	if err := json.Unmarshal([]byte(*raw), &providers); err != nil {
		return nil, fmt.Errorf("unable to parse identity providers %q: %w", *raw, err)
	}
	return providers, nil
}
