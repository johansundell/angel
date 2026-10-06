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
	out, err := exec.Command("make", "-s", "--eval=print-repo: ; @echo $(GHACCOUNT)/$(NAME)", "print-repo").Output()
	if err != nil {
		t.Fatalf("read GHACCOUNT and NAME from the Makefile: %v", err)
	}
	image := "ghcr.io/" + strings.TrimSpace(string(out))

	out, err = exec.Command("make", "-n", "docker-push", "VERSION=v9.9.9").CombinedOutput()
	if err != nil {
		t.Fatalf("make -n docker-push: %v\n%s", err, out)
	}
	cmds := string(out)

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
