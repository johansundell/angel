package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/johansundell/angel/types"
	"github.com/johansundell/angel/utils"
	"github.com/joho/godotenv"
)

var settings types.AppSettings

func loadSettings(filenames ...string) {
	// Load or reload .env file (Load preserves already set environment variables)
	if len(filenames) > 0 {
		if err := godotenv.Load(filenames...); err != nil {
			log.Println("No .env file found, using default/environment values")
		}
	} else {
		if err := godotenv.Load(); err != nil {
			if exe, exeErr := os.Executable(); exeErr == nil {
				envPath := filepath.Join(filepath.Dir(exe), ".env")
				if err = godotenv.Load(envPath); err != nil {
					log.Println("No .env file found, using default/environment values")
				}
			} else {
				log.Println("No .env file found, using default/environment values")
			}
		}
	}

	settings = types.AppSettings{}

	settings.Debug, _ = strconv.ParseBool(os.Getenv("DEBUG"))
	settings.Port = os.Getenv("PORT")
	if settings.Port == "" {
		settings.Port = ":8080"
	}
	settings.UseFileSystem, _ = strconv.ParseBool(os.Getenv("USE_FILE_SYSTEM"))

	timeoutStr := os.Getenv("TIMEOUT")
	if timeoutStr != "" {
		// TIMEOUT is whole seconds; an invalid value becomes 0, which Validate rejects.
		seconds, _ := strconv.Atoi(timeoutStr)
		settings.Timeout = time.Duration(seconds) * time.Second
	} else {
		settings.Timeout = 15 * time.Second
	}

	settings.SqlitePath = os.Getenv("SQLITE_PATH")
	if settings.SqlitePath == "" {
		settings.SqlitePath = filepath.Join(utils.GetBinaryBasePath(), nameOfService+".db")
	}

	settings.PIN.CaregiverPIN = strings.TrimSpace(os.Getenv("CAREGIVER_PIN"))
	settings.PIN.MasterPIN = strings.TrimSpace(os.Getenv("MASTER_PIN"))
	// Invalid durations become 0, which Validate rejects.
	settings.PIN.CaregiverSessionTTL = 20 * time.Minute
	if v := os.Getenv("CAREGIVER_SESSION_TIMEOUT"); v != "" {
		settings.PIN.CaregiverSessionTTL, _ = time.ParseDuration(v)
	} else if v := os.Getenv("SESSION_TIMEOUT"); v != "" {
		// Deprecated name for CAREGIVER_SESSION_TIMEOUT.
		settings.PIN.CaregiverSessionTTL, _ = time.ParseDuration(v)
		settings.PIN.LegacySessionTimeout = true
	}
	settings.PIN.ClientSessionTTL = 8 * time.Hour
	if v := os.Getenv("CLIENT_SESSION_TIMEOUT"); v != "" {
		settings.PIN.ClientSessionTTL, _ = time.ParseDuration(v)
	}
	settings.PIN.SessionSecret = os.Getenv("SESSION_SECRET")
	// Secure by default: the service is meant to be reached over HTTPS.
	settings.PIN.SecureCookie = true
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		settings.PIN.SecureCookie, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("SHARE_CAREGIVER_NAMES"); v != "" {
		// Unlike the other switches, a typo must not quietly hide or share
		// names, so Validate refuses a value that isn't a boolean.
		share, err := strconv.ParseBool(v)
		if err != nil {
			settings.PIN.InvalidShareCaregiverNames = v
		}
		settings.PIN.ShareCaregiverNames = share
	}
	for _, p := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			settings.TrustedProxies = append(settings.TrustedProxies, p)
		}
	}

}
