package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johansundell/angel/types"
	"github.com/johansundell/angel/utils"
)

func unsetEnv(keys ...string) func() {
	orig := make(map[string]string)
	has := make(map[string]bool)
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			orig[k] = v
			has[k] = true
		}
		os.Unsetenv(k)
	}
	return func() {
		for _, k := range keys {
			if has[k] {
				os.Setenv(k, orig[k])
			} else {
				os.Unsetenv(k)
			}
		}
	}
}

func TestLoadSettings(t *testing.T) {
	defer unsetEnv("PORT", "DEBUG")()

	// Create a temporary env file
	tmpEnv, err := os.CreateTemp("", ".env.*")
	if err != nil {
		t.Fatalf("Failed to create temp env file: %v", err)
	}
	defer os.Remove(tmpEnv.Name())

	_, err = tmpEnv.WriteString("PORT=:9999\nDEBUG=false\n")
	if err != nil {
		t.Fatalf("Failed to write to temp env file: %v", err)
	}
	tmpEnv.Close()

	loadSettings(tmpEnv.Name())

	if settings.Port != ":9999" {
		t.Errorf("expected settings.Port :9999, got %s", settings.Port)
	}
	if settings.Debug != false {
		t.Errorf("expected settings.Debug false, got %v", settings.Debug)
	}

	// Restore original settings
	loadSettings()
}

func TestLoadSettings_Precedence(t *testing.T) {
	t.Setenv("PORT", ":8888") // Real environment

	tmpEnv, err := os.CreateTemp("", ".env.*")
	if err != nil {
		t.Fatalf("Failed to create temp env file: %v", err)
	}
	defer os.Remove(tmpEnv.Name())

	_, err = tmpEnv.WriteString("PORT=:9999\n") // .env file
	if err != nil {
		t.Fatalf("Failed to write to temp env file: %v", err)
	}
	tmpEnv.Close()

	loadSettings(tmpEnv.Name())

	if settings.Port != ":8888" {
		t.Errorf("expected settings.Port :8888 (real env wins), got %s", settings.Port)
	}
}

func TestLoadSettings_NoDefaultAuthToken(t *testing.T) {
	defer unsetEnv("AUTH_TOKEN", "PORT")()

	tmpEnv, err := os.CreateTemp("", ".env.*")
	if err != nil {
		t.Fatalf("Failed to create temp env file: %v", err)
	}
	defer os.Remove(tmpEnv.Name())

	_, err = tmpEnv.WriteString("PORT=:9999\n")
	if err != nil {
		t.Fatalf("Failed to write to temp env file: %v", err)
	}
	tmpEnv.Close()

	loadSettings(tmpEnv.Name())

	if settings.AuthToken != "" {
		t.Errorf("expected empty settings.AuthToken, got %q", settings.AuthToken)
	}

	// Restore original settings
	loadSettings()
}

func TestLoadSettings_MySQLPort(t *testing.T) {
	testPort := func(t *testing.T, content string) string {
		defer unsetEnv("MYSQL_PORT")()
		tmpEnv, err := os.CreateTemp("", ".env.*")
		if err != nil {
			t.Fatalf("Failed to create temp env file: %v", err)
		}
		defer os.Remove(tmpEnv.Name())

		if _, err := tmpEnv.WriteString(content); err != nil {
			t.Fatalf("Failed to write to temp env file: %v", err)
		}
		tmpEnv.Close()

		loadSettings(tmpEnv.Name())
		return settings.MySQL.Port
	}

	t.Run("port with leading colon", func(t *testing.T) {
		got := testPort(t, "MYSQL_PORT=:3306\n")
		if got != "3306" {
			t.Errorf("expected settings.MySQL.Port '3306', got %q", got)
		}
	})

	t.Run("port without leading colon", func(t *testing.T) {
		got := testPort(t, "MYSQL_PORT=3307\n")
		if got != "3307" {
			t.Errorf("expected settings.MySQL.Port '3307', got %q", got)
		}
	})

	t.Run("default port when unset", func(t *testing.T) {
		got := testPort(t, "DEBUG=true\n")
		if got != "3306" {
			t.Errorf("expected default settings.MySQL.Port '3306', got %q", got)
		}
	})

	// Restore original settings
	loadSettings()
}

func TestLoadSettings_SqlitePath(t *testing.T) {
	testPath := func(t *testing.T, content string) string {
		defer unsetEnv("SQLITE_PATH")()
		tmpEnv, err := os.CreateTemp("", ".env.*")
		if err != nil {
			t.Fatalf("Failed to create temp env file: %v", err)
		}
		defer os.Remove(tmpEnv.Name())

		if _, err := tmpEnv.WriteString(content); err != nil {
			t.Fatalf("Failed to write to temp env file: %v", err)
		}
		tmpEnv.Close()

		loadSettings(tmpEnv.Name())
		return settings.SqlitePath
	}

	t.Run("custom sqlite path", func(t *testing.T) {
		tmpDir := t.TempDir()
		customPath := filepath.Join(tmpDir, "app.db")
		got := testPath(t, "SQLITE_PATH="+customPath+"\n")
		if got != customPath {
			t.Errorf("expected settings.SqlitePath %q, got %q", customPath, got)
		}
	})

	t.Run("default sqlite path when unset", func(t *testing.T) {
		got := testPath(t, "DEBUG=true\n")
		expected := filepath.Join(utils.GetBinaryBasePath(), nameOfService+".db")
		if got != expected {
			t.Errorf("expected default settings.SqlitePath %q, got %q", expected, got)
		}
	})
}

func TestLoadSettings_Storage(t *testing.T) {
	loadWith := func(t *testing.T, content string) string {
		defer unsetEnv("STORAGE")()
		tmpEnv, err := os.CreateTemp("", ".env.*")
		if err != nil {
			t.Fatalf("Failed to create temp env file: %v", err)
		}
		defer os.Remove(tmpEnv.Name())
		if _, err := tmpEnv.WriteString(content); err != nil {
			t.Fatalf("Failed to write to temp env file: %v", err)
		}
		tmpEnv.Close()

		loadSettings(tmpEnv.Name())
		return settings.Storage
	}
	defer loadSettings()

	if got := loadWith(t, "PORT=:9999\n"); got != types.StorageSQLite {
		t.Errorf("expected default storage %q, got %q", types.StorageSQLite, got)
	}
	if got := loadWith(t, "STORAGE= MySQL \n"); got != types.StorageMySQL {
		t.Errorf("expected STORAGE to be trimmed and lower-cased to %q, got %q", types.StorageMySQL, got)
	}
	if got := loadWith(t, "STORAGE=postgres\n"); got != "postgres" {
		t.Errorf("expected unknown storage to be kept for validation, got %q", got)
	}
	if err := settings.Validate(); err == nil {
		t.Error("expected Validate to reject an unknown STORAGE")
	}
}

func TestLoadSettings_FileMaker(t *testing.T) {
	load := func(t *testing.T, content string) types.FileMakerSettings {
		defer unsetEnv("FMS_HOST", "FMS_DATABASE", "FMS_USERNAME", "FMS_PASSWORD", "FMS_TIMEOUT", "FMS_LOG_TABLE", "FMS_CA_FILE", "FMS_INSECURE_SKIP_VERIFY", "STORAGE")()
		tmpEnv, err := os.CreateTemp("", ".env.*")
		if err != nil {
			t.Fatalf("Failed to create temp env file: %v", err)
		}
		defer os.Remove(tmpEnv.Name())
		tmpEnv.WriteString(content)
		tmpEnv.Close()
		loadSettings(tmpEnv.Name())
		return settings.FileMaker
	}
	defer loadSettings()

	fm := load(t, "PORT=:9999\n")
	if fm.Timeout != 10*time.Second || fm.LogTable != "Logs" || fm.InsecureSkipVerify {
		t.Errorf("unexpected defaults: %+v", fm)
	}

	fm = load(t, "CAREGIVER_PIN=1234\nMASTER_PIN=987654\nSTORAGE=filemaker\nFMS_HOST=https://fms.example.com\nFMS_DATABASE=Logging\nFMS_USERNAME=u\nFMS_PASSWORD=p\nFMS_TIMEOUT=3s\nFMS_LOG_TABLE=ServiceLogs\nFMS_CA_FILE=/etc/ca.pem\nFMS_INSECURE_SKIP_VERIFY=true\n")
	want := types.FileMakerSettings{Host: "https://fms.example.com", Database: "Logging", Username: "u", Password: "p", Timeout: 3 * time.Second, LogTable: "ServiceLogs", CAFile: "/etc/ca.pem", InsecureSkipVerify: true}
	if fm != want {
		t.Errorf("expected %+v, got %+v", want, fm)
	}
	if err := settings.Validate(); err != nil {
		t.Errorf("expected loaded FileMaker settings to validate, got %v", err)
	}

	if fm = load(t, "FMS_TIMEOUT=soon\n"); fm.Timeout != 0 {
		t.Errorf("expected an invalid FMS_TIMEOUT to become 0 for Validate to reject, got %v", fm.Timeout)
	}
}

func TestLoadSettings_Timeout(t *testing.T) {
	load := func(content string) time.Duration {
		defer unsetEnv("TIMEOUT")()
		tmpEnv, err := os.CreateTemp("", ".env.*")
		if err != nil {
			t.Fatalf("Failed to create temp env file: %v", err)
		}
		defer os.Remove(tmpEnv.Name())
		tmpEnv.WriteString(content)
		tmpEnv.Close()
		loadSettings(tmpEnv.Name())
		return settings.Timeout
	}
	defer loadSettings()

	if got := load("PORT=:9999\n"); got != 15*time.Second {
		t.Errorf("expected default TIMEOUT 15s, got %v", got)
	}
	if got := load("TIMEOUT=20\n"); got != 20*time.Second {
		t.Errorf("expected TIMEOUT=20 to mean 20s, got %v", got)
	}
	if got := load("TIMEOUT=soon\n"); got != 0 {
		t.Errorf("expected an invalid TIMEOUT to become 0 for Validate to reject, got %v", got)
	}
}

func TestLoadSettings_PIN(t *testing.T) {
	keys := []string{"CAREGIVER_PIN", "MASTER_PIN", "CAREGIVER_SESSION_TIMEOUT", "CLIENT_SESSION_TIMEOUT", "SESSION_TIMEOUT", "SESSION_SECRET", "COOKIE_SECURE", "TRUSTED_PROXIES"}
	defer unsetEnv(keys...)()
	defer loadSettings()

	load := func(t *testing.T, content string) types.AppSettings {
		t.Helper()
		defer unsetEnv(keys...)()
		path := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		loadSettings(path)
		return settings
	}

	s := load(t, "PORT=:9999\n")
	if s.PIN.CaregiverSessionTTL != 20*time.Minute || s.PIN.ClientSessionTTL != 8*time.Hour || s.PIN.LegacySessionTimeout || !s.PIN.SecureCookie || s.TrustedProxies != nil {
		t.Errorf("unexpected defaults: %+v, proxies %v", s.PIN, s.TrustedProxies)
	}

	s = load(t, "CAREGIVER_PIN=1234\nMASTER_PIN= 987654 \nCAREGIVER_SESSION_TIMEOUT=25m\nCLIENT_SESSION_TIMEOUT=12h\nSESSION_SECRET=abc\nCOOKIE_SECURE=false\nTRUSTED_PROXIES=127.0.0.1, 10.0.0.0/8,\n")
	want := types.PINSettings{CaregiverPIN: "1234", MasterPIN: "987654", CaregiverSessionTTL: 25 * time.Minute, ClientSessionTTL: 12 * time.Hour, SessionSecret: "abc", SecureCookie: false}
	if s.PIN != want {
		t.Errorf("expected %+v, got %+v", want, s.PIN)
	}
	if len(s.TrustedProxies) != 2 || s.TrustedProxies[0] != "127.0.0.1" || s.TrustedProxies[1] != "10.0.0.0/8" {
		t.Errorf("unexpected trusted proxies %q", s.TrustedProxies)
	}

	if s = load(t, "CAREGIVER_SESSION_TIMEOUT=soon\nCLIENT_SESSION_TIMEOUT=later\n"); s.PIN.CaregiverSessionTTL != 0 || s.PIN.ClientSessionTTL != 0 {
		t.Errorf("invalid session timeouts should become 0 for Validate to reject, got %v and %v", s.PIN.CaregiverSessionTTL, s.PIN.ClientSessionTTL)
	}

	// The deprecated name still sets the Caregiver lifetime when the new one
	// is unset, and is flagged so startup can warn.
	if s = load(t, "SESSION_TIMEOUT=25m\n"); s.PIN.CaregiverSessionTTL != 25*time.Minute || !s.PIN.LegacySessionTimeout {
		t.Errorf("SESSION_TIMEOUT alone: got %v, legacy %v", s.PIN.CaregiverSessionTTL, s.PIN.LegacySessionTimeout)
	}
	if s = load(t, "SESSION_TIMEOUT=25m\nCAREGIVER_SESSION_TIMEOUT=17m\n"); s.PIN.CaregiverSessionTTL != 17*time.Minute || s.PIN.LegacySessionTimeout {
		t.Errorf("both names set: got %v, legacy %v; CAREGIVER_SESSION_TIMEOUT should win", s.PIN.CaregiverSessionTTL, s.PIN.LegacySessionTimeout)
	}
}

func TestLoadSettings_ShareCaregiverNames(t *testing.T) {
	defer unsetEnv("SHARE_CAREGIVER_NAMES")()
	defer loadSettings()

	// load reads content as .env, with SHARE_CAREGIVER_NAMES also set in the
	// environment when env is not nil, and restores the environment before
	// returning.
	load := func(t *testing.T, content string, env *string) types.AppSettings {
		t.Helper()
		defer unsetEnv("SHARE_CAREGIVER_NAMES")()
		if env != nil {
			os.Setenv("SHARE_CAREGIVER_NAMES", *env)
		}
		path := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		loadSettings(path)
		s := settings
		s.PIN.CaregiverPIN, s.PIN.MasterPIN = "1234", "98765"
		return s
	}
	str := func(s string) *string { return &s }

	for _, tc := range []struct {
		name    string
		envFile string
		env     *string
		want    bool
		wantErr bool
	}{
		{name: "unset", want: false},
		{name: "empty", envFile: "SHARE_CAREGIVER_NAMES=\n", want: false},
		{name: "true in .env", envFile: "SHARE_CAREGIVER_NAMES=true\n", want: true},
		{name: "false in .env", envFile: "SHARE_CAREGIVER_NAMES=false\n", want: false},
		{name: "true in environment", env: str("true"), want: true},
		{name: "environment wins over .env", envFile: "SHARE_CAREGIVER_NAMES=true\n", env: str("false"), want: false},
		{name: "invalid", envFile: "SHARE_CAREGIVER_NAMES=yes\n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := load(t, tc.envFile, tc.env)

			if s.PIN.ShareCaregiverNames != tc.want {
				t.Errorf("ShareCaregiverNames = %v, want %v", s.PIN.ShareCaregiverNames, tc.want)
			}
			err := s.Validate()
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "SHARE_CAREGIVER_NAMES") {
					t.Errorf("Validate() = %v, want an error naming SHARE_CAREGIVER_NAMES", err)
				}
			} else if err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}
