package auth

import (
	"strings"
	"testing"
	"time"
)

func TestNew_RejectsUnsafeConfig(t *testing.T) {
	valid := func() Config {
		return Config{CaregiverPIN: "1234", MasterPIN: "987654", Secret: []byte(strings.Repeat("s", 32)), SessionTTL: 20 * time.Minute}
	}
	tests := []struct {
		name   string
		modify func(*Config)
	}{
		{"missing caregiver PIN", func(c *Config) { c.CaregiverPIN = "" }},
		{"missing master PIN", func(c *Config) { c.MasterPIN = "" }},
		{"same PINs", func(c *Config) { c.MasterPIN = c.CaregiverPIN }},
		{"short secret", func(c *Config) { c.Secret = []byte("short") }},
		{"no TTL", func(c *Config) { c.SessionTTL = 0 }},
	}
	if _, err := New(valid()); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid()
			tc.modify(&cfg)
			if _, err := New(cfg); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestLimiter_PrunesFinishedWindows(t *testing.T) {
	l := newLimiter(2, time.Minute)
	now := time.Unix(0, 0)
	for i := 0; i < 100; i++ {
		l.fail(string(rune('a'+i)), now)
	}
	l.allow("x", now.Add(time.Minute))
	if len(l.failures) != 0 {
		t.Fatalf("expected finished windows to be pruned, %d left", len(l.failures))
	}
}
