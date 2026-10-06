package types

import (
	"strings"
	"testing"
	"time"
)

var validPIN = PINSettings{CaregiverPIN: "1234", MasterPIN: "987654", CaregiverSessionTTL: 20 * time.Minute, ClientSessionTTL: 8 * time.Hour}

func TestAppSettingsValidate(t *testing.T) {
	tests := []struct {
		name    string
		s       AppSettings
		wantErr bool
	}{
		{"valid defaults", AppSettings{PIN: validPIN, Port: ":8080", Timeout: 10 * time.Second, Storage: StorageSQLite}, false},
		{"missing storage", AppSettings{PIN: validPIN, Port: ":8080", Timeout: 10 * time.Second}, true},
		{"unknown storage", AppSettings{PIN: validPIN, Port: ":8080", Timeout: 10 * time.Second, Storage: "postgres"}, true},
		{"missing port", AppSettings{PIN: validPIN, Port: "", Timeout: 10 * time.Second, Storage: StorageSQLite}, true},
		{"invalid timeout", AppSettings{PIN: validPIN, Port: ":8080", Timeout: 0, Storage: StorageSQLite}, true},
		{"mysql missing fields", AppSettings{PIN: validPIN, Port: ":8080", Timeout: 10 * time.Second, Storage: StorageMySQL, MySQL: struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Host     string `json:"host"`
			Port     string `json:"port"`
			Database string `json:"database"`
		}{}}, true},
		{"mysql provided", AppSettings{PIN: validPIN, Port: ":8080", Timeout: 10 * time.Second, Storage: StorageMySQL, MySQL: struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Host     string `json:"host"`
			Port     string `json:"port"`
			Database string `json:"database"`
		}{Username: "u", Host: "localhost", Database: "db"}}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.s.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestAppSettingsValidate_FileMaker(t *testing.T) {
	valid := func() AppSettings {
		return AppSettings{PIN: validPIN, Port: ":8080", Timeout: 10 * time.Second, Storage: StorageFileMaker, FileMaker: FileMakerSettings{
			Host: "https://fms.example.com", Database: "Logging", Username: "u", Password: "p", Timeout: 10 * time.Second, LogTable: "Logs",
		}}
	}
	tests := []struct {
		name    string
		modify  func(*AppSettings)
		wantErr string
	}{
		{"valid", func(*AppSettings) {}, ""},
		{"missing host", func(s *AppSettings) { s.FileMaker.Host = "" }, "FMS_HOST"},
		{"missing password", func(s *AppSettings) { s.FileMaker.Password = "" }, "FMS_PASSWORD"},
		{"plain http", func(s *AppSettings) { s.FileMaker.Host = "http://fms.example.com" }, "https://"},
		{"no timeout", func(s *AppSettings) { s.FileMaker.Timeout = 0 }, "FMS_TIMEOUT"},
		{"empty table", func(s *AppSettings) { s.FileMaker.LogTable = "" }, "FMS_LOG_TABLE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := valid()
			tc.modify(&s)
			err := s.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected an error mentioning %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestPINSettingsValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*PINSettings)
		wantErr string
	}{
		{"valid", func(*PINSettings) {}, ""},
		{"caregiver missing", func(p *PINSettings) { p.CaregiverPIN = "" }, "CAREGIVER_PIN"},
		{"caregiver too long", func(p *PINSettings) { p.CaregiverPIN = "12345" }, "CAREGIVER_PIN"},
		{"caregiver not digits", func(p *PINSettings) { p.CaregiverPIN = "12a4" }, "CAREGIVER_PIN"},
		{"master missing", func(p *PINSettings) { p.MasterPIN = "" }, "MASTER_PIN"},
		{"master too long", func(p *PINSettings) { p.MasterPIN = "1234567890123" }, "MASTER_PIN"},
		{"master not digits", func(p *PINSettings) { p.MasterPIN = "98765x" }, "MASTER_PIN"},
		{"same PINs", func(p *PINSettings) { p.MasterPIN = p.CaregiverPIN }, "must differ"},
		{"caregiver ttl too short", func(p *PINSettings) { p.CaregiverSessionTTL = 14 * time.Minute }, "CAREGIVER_SESSION_TIMEOUT"},
		{"caregiver ttl too long", func(p *PINSettings) { p.CaregiverSessionTTL = 31 * time.Minute }, "CAREGIVER_SESSION_TIMEOUT"},
		{"caregiver ttl min", func(p *PINSettings) { p.CaregiverSessionTTL = MinCaregiverSessionTTL }, ""},
		{"caregiver ttl max", func(p *PINSettings) { p.CaregiverSessionTTL = MaxCaregiverSessionTTL }, ""},
		{"client ttl too short", func(p *PINSettings) { p.ClientSessionTTL = 14 * time.Minute }, "CLIENT_SESSION_TIMEOUT"},
		{"client ttl too long", func(p *PINSettings) { p.ClientSessionTTL = 24*time.Hour + time.Second }, "CLIENT_SESSION_TIMEOUT"},
		{"client ttl unset", func(p *PINSettings) { p.ClientSessionTTL = 0 }, "CLIENT_SESSION_TIMEOUT"},
		{"client ttl min", func(p *PINSettings) { p.ClientSessionTTL = MinClientSessionTTL }, ""},
		{"client ttl max", func(p *PINSettings) { p.ClientSessionTTL = MaxClientSessionTTL }, ""},
		{"short secret", func(p *PINSettings) { p.SessionSecret = "short" }, "SESSION_SECRET"},
		{"long secret", func(p *PINSettings) { p.SessionSecret = strings.Repeat("s", 32) }, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validPIN
			tc.modify(&p)
			err := p.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}

	// An out-of-range value from the deprecated name is reported by that name.
	legacy := validPIN
	legacy.CaregiverSessionTTL, legacy.LegacySessionTimeout = time.Hour, true
	if err := legacy.Validate(); err == nil || !strings.HasPrefix(err.Error(), "SESSION_TIMEOUT ") {
		t.Fatalf("expected an error naming SESSION_TIMEOUT, got %v", err)
	}

	s := AppSettings{Port: ":8080", Timeout: 10 * time.Second, Storage: StorageSQLite}
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "CAREGIVER_PIN") {
		t.Fatalf("AppSettings.Validate must reject missing PINs, got %v", err)
	}
}
