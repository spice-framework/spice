package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	releaseWorkflowRevision = "fde3eccd4770233904f9adca0ebd35687f119d0e"
	releaseVersion          = "v0.1.0-preview.4"
)

func checkReleaseContract(root string) error {
	if err := checkVerifyReleaseTarget(root); err != nil {
		return err
	}
	if err := checkReleaseWorkflow(root); err != nil {
		return err
	}
	return checkReleaseIntent(root)
}

func checkVerifyReleaseTarget(root string) error {
	content, err := os.ReadFile(filepath.Join(root, "Makefile")) // #nosec G304 -- root and Makefile path are repository-owned.
	if err != nil {
		return fmt.Errorf("read Makefile release target: %w", err)
	}
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	want := "\nverify-release:\n\tgo run ./internal/qualitygate -mode=verify-release\n"
	if strings.Count(normalized, want) != 1 {
		return errors.New("verify-release target must invoke the unconditional core quality gate exactly once")
	}
	return nil
}

func checkReleaseWorkflow(root string) error {
	path := filepath.Join(root, ".github", "workflows", "release.yml")
	content, err := os.ReadFile(path) // #nosec G304 -- root and workflow path are repository-owned.
	if err != nil {
		return fmt.Errorf("read release workflow: %w", err)
	}
	if strings.ReplaceAll(string(content), "\r\n", "\n") != expectedReleaseWorkflow(modulePath) {
		return fmt.Errorf(
			"release workflow must call the exact keyless central workflow at %s for module %s with only the required permission ceiling and no secrets",
			releaseWorkflowRevision,
			modulePath,
		)
	}
	return nil
}

func expectedReleaseWorkflow(module string) string {
	return fmt.Sprintf(`name: Release

on:
  push:
    tags:
      - "v[0-9]*.[0-9]*.[0-9]*"

permissions: {}

jobs:
  release:
    name: Keylessly attest and publish
    permissions:
      contents: write
      id-token: write
      attestations: write
      artifact-metadata: write
    uses: spice-framework/.github/.github/workflows/go-module-release.yml@%s
    with:
      module: %s
      workflow_commit: %s
`, releaseWorkflowRevision, module, releaseWorkflowRevision)
}

func checkReleaseIntent(root string) error {
	path := filepath.Join(root, "spice-release.json")
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect release intent: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("release intent must be a regular file, not a symlink")
	}
	content, err := os.ReadFile(path) // #nosec G304 -- root and release-intent path are repository-owned.
	if err != nil {
		return fmt.Errorf("read release intent: %w", err)
	}
	if string(content) != expectedReleaseIntent() {
		return errors.New("release intent must be the exact canonical go-module-v1 preview.4 identity")
	}
	return nil
}

func expectedReleaseIntent() string {
	return fmt.Sprintf(`{
  "schema": 1,
  "profile": "go-module-v1",
  "repository": "spice",
  "module": %q,
  "version": %q
}
`, modulePath, releaseVersion)
}
