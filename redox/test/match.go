package test

import (
	"github.com/tidepool-org/clinic/client"
	"go.uber.org/mock/gomock"
)

type ArgMatcher[T any] struct {
	MatchFn func(T) bool
}

func (a ArgMatcher[T]) String() string {
	return "matches argument"
}

func (a ArgMatcher[T]) Matches(arg interface{}) bool {
	targ, ok := arg.(T)
	if !ok {
		return false
	}
	return a.MatchFn(targ)
}

func MatchArg[T any](fn func(T) bool) gomock.Matcher {
	return ArgMatcher[T]{MatchFn: fn}
}

func PatientHasTags(actual *client.PatientTagIds, expected []string) bool {
	if actual == nil {
		return false
	}
	if len(*actual) == 0 {
		return false
	}

	expectedMap := map[string]struct{}{}
	for _, e := range expected {
		expectedMap[e] = struct{}{}
	}

	for _, t := range *actual {
		if _, ok := expectedMap[t]; ok {
			delete(expectedMap, t)
		}
	}

	return len(expectedMap) == 0
}
