package test

import "github.com/tidepool-org/go-common/clients/shoreline"

// Never returns a user when GetUser is called
type ShorelineNoUser struct {
	shoreline.Client
}

func (s *ShorelineNoUser) GetUser(userID, token string) (*shoreline.UserData, error) {
	return nil, nil
}
