package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigAllowsEmptyFile(t *testing.T) {
	for _, content := range []string{"", " \r\n\t"} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("APP_CONFIG_PATH", path)

			config, err := loadConfig()
			if err != nil {
				t.Fatalf("empty config should use defaults: %v", err)
			}
			if !config.RegistrationEnabled || !config.GuestCheckoutEnabled || !config.BalancePaymentEnabled || !config.OnlinePaymentEnabled {
				t.Fatalf("default switches not applied: %+v", config)
			}
		})
	}
}

func TestLoadConfigRejectsMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"registration_enabled":`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_CONFIG_PATH", path)

	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "parse config file") {
		t.Fatalf("malformed config should fail with parse error: %v", err)
	}
}
