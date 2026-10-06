package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// httpsServerFiles are the files a server needs for docker-compose.https.yml,
// besides .env (README, "Running with HTTPS").
var httpsServerFiles = []string{"docker-compose.https.yml", "Caddyfile"}

// composeHTTPSConfig copies the server files and the given .env into an empty
// folder, as on a server without a checkout, and returns the angel service
// from docker compose config there.
func composeHTTPSConfig(t *testing.T, env string) map[string]any {
	t.Helper()
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		t.Skip("docker compose not installed")
	}
	dir := t.TempDir()
	for _, name := range httpsServerFiles {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("docker", "compose", "-f", "docker-compose.https.yml", "config", "--format", "json")
	cmd.Dir = dir
	// Only .env decides; a VERSION in the test's environment would win over it.
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "VERSION=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("docker compose config: %v\n%s", err, stderr)
	}
	var cfg struct {
		Services map[string]map[string]any `json:"services"`
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("parse compose config: %v", err)
	}
	angel, ok := cfg.Services["angel"]
	if !ok {
		t.Fatalf("no angel service in:\n%s", out)
	}
	return angel
}

// TestComposeHTTPSPullsPublishedImage checks that the HTTPS stack runs the
// published image without a checkout: no build, and VERSION in .env picks the
// tag, with latest when it is unset or empty.
func TestComposeHTTPSPullsPublishedImage(t *testing.T) {
	const image = "ghcr.io/johansundell/angel"
	for _, tc := range []struct {
		name, env, want string
	}{
		{"unset", "DOMAIN=angel.example.com\n", image + ":latest"},
		{"empty", "DOMAIN=angel.example.com\nVERSION=\"\"\n", image + ":latest"},
		{"set", "DOMAIN=angel.example.com\nVERSION=v1.2.3\n", image + ":v1.2.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			angel := composeHTTPSConfig(t, tc.env)
			if got := angel["image"]; got != tc.want {
				t.Errorf("image = %v, want %s", got, tc.want)
			}
			if b, ok := angel["build"]; ok {
				t.Errorf("angel builds (%v); a server has no checkout to build from", b)
			}
		})
	}
}

// TestMakeDockerRunHTTPSPulls checks that make docker-run-https pulls instead
// of building, and leaves the tag to VERSION in .env rather than the Makefile
// VERSION, which may not be published.
func TestMakeDockerRunHTTPSPulls(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not installed")
	}
	cmds := makeDryRun(t, "docker-run-https")
	if strings.Contains(cmds, "--build") {
		t.Errorf("make docker-run-https builds; it runs:\n%s", cmds)
	}
	if !strings.Contains(cmds, "--pull always") {
		t.Errorf("make docker-run-https doesn't pull; it runs:\n%s", cmds)
	}
	if !strings.Contains(cmds, "env -u VERSION") {
		t.Errorf("make docker-run-https passes the Makefile VERSION on; it runs:\n%s", cmds)
	}
}

// TestMakeDockerBuildHTTPSBuilds checks that the checkout route still exists:
// make docker-build-https builds Angel with the Makefile VERSION.
func TestMakeDockerBuildHTTPSBuilds(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not installed")
	}
	cmds := makeDryRun(t, "docker-build-https", "VERSION=v9.9.9")
	for _, want := range []string{
		"VERSION=v9.9.9",
		"-f docker-compose.https.yml -f docker-compose.https.build.yml",
		"--build",
	} {
		if !strings.Contains(cmds, want) {
			t.Errorf("make docker-build-https doesn't run %q; it runs:\n%s", want, cmds)
		}
	}
}
