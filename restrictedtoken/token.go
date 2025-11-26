package token

import (
	"fmt"
	"reflect"
	"time"

	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
)

const (
	RestrictedTokenExpirationDuration = time.Hour * 24 * 30
)

func CreateRestrictedTokenForProvider(auth clients.AuthClient, shoreline shoreline.Client, userId string, providerName string) (*clients.RestrictedToken, error) {
	restrictedTokenPaths := []string{"/v1/oauth/" + providerName}
	restrictedTokenExpirationTime := time.Now().Add(RestrictedTokenExpirationDuration)

	currentRestrictedTokens, err := auth.ListUserRestrictedTokens(userId, shoreline.TokenProvide())
	if err != nil {
		return nil, fmt.Errorf(`error fetching user restricted tokens: %w`, err)
	}

	var currentRestrictedTokenId string
	for _, token := range currentRestrictedTokens {
		if reflect.DeepEqual(token.Paths, &restrictedTokenPaths) {
			currentRestrictedTokenId = token.ID
			break
		}
	}

	// Revoke all existing tokens and re-create them to make sure old ones are not valid
	// in case the email of the patient changed
	if currentRestrictedTokenId != "" {
		err := auth.DeleteRestrictedToken(currentRestrictedTokenId, shoreline.TokenProvide())
		if err != nil {
			return nil, fmt.Errorf(`error deleting restricted token: %w`, err)
		}
	}
	return auth.CreateRestrictedToken(userId, restrictedTokenExpirationTime, restrictedTokenPaths, shoreline.TokenProvide())
}
