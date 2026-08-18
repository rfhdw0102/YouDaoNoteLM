package config_test

import (
	"testing"

	"YoudaoNoteLm/pkg/config"
)

func TestNotionConfigConfiguredRequiresOAuthFields(t *testing.T) {
	cfg := config.NotionConfig{
		ClientID:            "id",
		ClientSecret:        "secret",
		RedirectURI:         "https://app.test/api/v1/notion/oauth/callback",
		FrontendRedirectURL: "https://app.test",
	}
	if !cfg.Configured() {
		t.Fatal("expected configured")
	}

	cfg.ClientSecret = ""
	if cfg.Configured() {
		t.Fatal("expected unconfigured without client secret")
	}
}
