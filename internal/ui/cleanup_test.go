package ui

import (
	"errors"
	"reflect"
	"testing"
)

func TestCleanupResourcesRunsEveryCleanupAndReturnsErrors(t *testing.T) {
	playerErr := errors.New("player cleanup")
	thumbErr := errors.New("thumbnail cleanup")
	var calls []string
	err := cleanupResources(
		func() error {
			calls = append(calls, "player")
			return playerErr
		},
		func() { calls = append(calls, "downloads") },
		func() error {
			calls = append(calls, "thumbnail")
			return thumbErr
		},
	)
	if !reflect.DeepEqual(calls, []string{"player", "downloads", "thumbnail"}) {
		t.Fatalf("cleanup order = %v", calls)
	}
	if !errors.Is(err, playerErr) || !errors.Is(err, thumbErr) {
		t.Fatalf("cleanup error = %v, want both resource errors", err)
	}
}

func TestCleanupResourcesSkipsUninitializedResources(t *testing.T) {
	if err := cleanupResources(nil, nil, nil); err != nil {
		t.Fatalf("cleanupResources() error = %v, want nil", err)
	}
}
