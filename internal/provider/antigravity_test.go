package provider_test

import (
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/provider"
)

func TestAntigravityProvider_Name(t *testing.T) {
	t.Parallel()
	if (provider.AntigravityProvider{}).Name() != "antigravity" {
		t.Errorf("Name: %q", (provider.AntigravityProvider{}).Name())
	}
}
