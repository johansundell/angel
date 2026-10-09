package main

import (
	"os/exec"
	"strings"
	"testing"
)

// TestMakeDockerPushTargetsGHCR checks that make docker-push builds the image
// with the Makefile version baked in and pushes the version and latest tags to
// the GitHub Container Registry, the same tags CI pushes for a v* git tag.
func TestMakeDockerPushTargetsGHCR(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not installed")
	}
	// Account and name come from the Makefile, so the test still passes after
	// the template rename changes GHACCOUNT.
	repo, err := exec.Command("make", "-s", "--eval=print-repo: ; @echo $(GHACCOUNT)/$(NAME)", "print-repo").Output()
	if err != nil {
		t.Fatalf("read GHACCOUNT and NAME from the Makefile: %v", err)
	}
	image := "ghcr.io/" + strings.TrimSpace(string(repo))

	cmds := makeDryRun(t, "docker-push", "VERSION=v9.9.9")

	for _, want := range []string{
		"--build-arg VERSION=v9.9.9",
		"-t " + image + ":v9.9.9",
		"-t " + image + ":latest",
		"docker push " + image + ":v9.9.9",
		"docker push " + image + ":latest",
	} {
		if !strings.Contains(cmds, want) {
			t.Errorf("make docker-push doesn't run %q; it runs:\n%s", want, cmds)
		}
	}
}

// TestMakeBumpRelease checks that make bump-release delegates to
// scripts/bump-release.sh with BUMP and DRY_RUN forwarded.
func TestMakeBumpRelease(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not installed")
	}
	cmds := makeDryRun(t, "bump-release", "BUMP=minor", "DRY_RUN=1")
	want := "./scripts/bump-release.sh minor"
	if !strings.Contains(cmds, want) {
		t.Errorf("make bump-release doesn't run %q; it runs:\n%s", want, cmds)
	}
}

// TestMakeBackup checks that make backup builds the binary and runs its
// backup subcommand with ARGS forwarded.
func TestMakeBackup(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not installed")
	}
	cmds := makeDryRun(t, "backup", "ARGS=-dir /srv/backups -retention-days 14")
	for _, want := range []string{
		"go build",
		"./angel backup -dir /srv/backups -retention-days 14",
	} {
		if !strings.Contains(cmds, want) {
			t.Errorf("make backup doesn't run %q; it runs:\n%s", want, cmds)
		}
	}
	if strings.Index(cmds, "go build") > strings.Index(cmds, "./angel backup") {
		t.Errorf("make backup must build before it runs the backup:\n%s", cmds)
	}
}

// makeDryRun returns the commands make would run for target, without running
// them.
func makeDryRun(t *testing.T, target string, args ...string) string {
	t.Helper()
	out, err := exec.Command("make", append([]string{"-n", target}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("make -n %s: %v\n%s", target, err, out)
	}
	return string(out)
}
