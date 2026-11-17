package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "defaults for optional variables",
			env:  map[string]string{"DATABASE_URL": "postgres://example"},
			want: Config{AppEnv: "development", Port: "8080", DatabaseURL: "postgres://example", AppBaseURL: "http://localhost:5173", PaymentProvider: "simulated"},
		},
		{
			name: "explicit values",
			env:  map[string]string{"DATABASE_URL": "postgres://example", "APP_ENV": "production", "PORT": "10000", "APP_BASE_URL": "https://shop.example/"},
			want: Config{AppEnv: "production", Port: "10000", DatabaseURL: "postgres://example", AppBaseURL: "https://shop.example", PaymentProvider: "simulated"},
		},
		{
			name: "stripe with test keys",
			env: map[string]string{"DATABASE_URL": "postgres://example", "APP_ENV": "production", "PAYMENT_PROVIDER": "stripe",
				"STRIPE_SECRET_KEY": "sk_test_abc", "STRIPE_WEBHOOK_SECRET": "whsec_abc"},
			want: Config{AppEnv: "production", Port: "8080", DatabaseURL: "postgres://example", AppBaseURL: "http://localhost:5173",
				PaymentProvider: "stripe", StripeSecretKey: "sk_test_abc", StripeWebhookSecret: "whsec_abc"},
		},
		{
			name:    "stripe live key is refused",
			env:     map[string]string{"DATABASE_URL": "postgres://example", "PAYMENT_PROVIDER": "stripe", "STRIPE_SECRET_KEY": "sk_live_abc"},
			wantErr: true,
		},
		{
			name:    "stripe in production without webhook secret",
			env:     map[string]string{"DATABASE_URL": "postgres://example", "APP_ENV": "production", "PAYMENT_PROVIDER": "stripe", "STRIPE_SECRET_KEY": "sk_test_abc"},
			wantErr: true,
		},
		{
			name:    "unknown provider",
			env:     map[string]string{"DATABASE_URL": "postgres://example", "PAYMENT_PROVIDER": "paypal"},
			wantErr: true,
		},
		{
			name:    "missing database url",
			env:     map[string]string{},
			wantErr: true,
		},
		{
			name:    "base url with a path",
			env:     map[string]string{"DATABASE_URL": "postgres://example", "APP_BASE_URL": "https://shop.example/store"},
			wantErr: true,
		},
		{
			name:    "base url without scheme",
			env:     map[string]string{"DATABASE_URL": "postgres://example", "APP_BASE_URL": "shop.example"},
			wantErr: true,
		},
		{
			name:    "unknown app env",
			env:     map[string]string{"DATABASE_URL": "postgres://example", "APP_ENV": "staging"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv restores the previous value when the subtest ends, so cases
			// cannot leak into each other. Every variable is set explicitly ("" when
			// absent) so the result does not depend on the developer's own shell.
			for _, key := range []string{"APP_ENV", "PORT", "DATABASE_URL", "APP_BASE_URL", "PAYMENT_PROVIDER", "STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET"} {
				t.Setenv(key, tt.env[key])
			}

			got, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLoadDotEnv(t *testing.T) {
	content := `# a comment
PLAIN=value

QUOTED="quoted value"
SINGLE='single'
WITH_EQUALS=postgres://u:p@host/db?sslmode=require
  SPACED  =  padded
ALREADY_SET=from file
not a valid line
`
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	keys := []string{"PLAIN", "QUOTED", "SINGLE", "WITH_EQUALS", "SPACED"}
	for _, key := range keys {
		// Register the restore with t.Setenv, then really unset the variable so
		// LoadDotEnv sees it as absent.
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
	t.Setenv("ALREADY_SET", "from environment")

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv() error = %v", err)
	}

	want := map[string]string{
		"PLAIN":       "value",
		"QUOTED":      "quoted value",
		"SINGLE":      "single",
		"WITH_EQUALS": "postgres://u:p@host/db?sslmode=require",
		"SPACED":      "padded",
		"ALREADY_SET": "from environment", // the real environment wins over the file
	}
	for key, wantValue := range want {
		if got := os.Getenv(key); got != wantValue {
			t.Errorf("%s = %q, want %q", key, got, wantValue)
		}
	}
}

func TestLoadDotEnvMissingFileIsNotAnError(t *testing.T) {
	if err := LoadDotEnv(filepath.Join(t.TempDir(), "does-not-exist")); err != nil {
		t.Errorf("LoadDotEnv() error = %v, want nil", err)
	}
}
