package token

import (
	"fmt"
	"slices"
	"time"

	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
)

const (
	RestrictedTokenExpirationDuration = time.Hour * 24 * 30
)

func UpsertRestrictedTokenForProvider(auth clients.AuthClient, shoreline shoreline.Client, userId string, providerName string) (*clients.RestrictedToken, error) {
	restrictedTokenPaths := []string{"/v1/oauth/" + providerName}
	restrictedTokenExpirationTime := time.Now().Add(RestrictedTokenExpirationDuration)

	currentRestrictedTokens, err := auth.ListUserRestrictedTokens(userId, shoreline.TokenProvide())
	if err != nil {
		return nil, fmt.Errorf(`error fetching user restricted tokens: %w`, err)
	}

	var existingTokenIDs []string
	for _, token := range currentRestrictedTokens {
		if token.Paths != nil && slices.Equal(*token.Paths, restrictedTokenPaths) {
			existingTokenIDs = append(existingTokenIDs, token.ID)
		}
	}

	var lastExistingToken *clients.RestrictedToken
	for _, tokenID := range existingTokenIDs {
		restrictedToken, err := auth.UpdateRestrictedToken(tokenID, restrictedTokenExpirationTime, restrictedTokenPaths, shoreline.TokenProvide())
		if err != nil {
			return nil, err
		}
		lastExistingToken = restrictedToken
	}

	if lastExistingToken != nil {
		return lastExistingToken, nil
	}
	return auth.CreateRestrictedToken(userId, restrictedTokenExpirationTime, restrictedTokenPaths, shoreline.TokenProvide())
}
