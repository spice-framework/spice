package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckVerifyReleaseTarget(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "exact target", content: "verify:\n\tgo run ./internal/qualitygate -mode=verify\n\nverify-release:\n\tgo run ./internal/qualitygate -mode=verify-release\n"},
		{name: "missing target", content: "verify:\n\tgo run ./internal/qualitygate -mode=verify\n", wantErr: true},
		{name: "reduced target", content: "verify-release:\n\tgo run ./internal/qualitygate -mode=check\n", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			err := checkVerifyReleaseTarget(root)
			if (err != nil) != test.wantErr {
				t.Fatalf("checkVerifyReleaseTarget() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestCheckReleaseWorkflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(string) string
		wantErr bool
	}{
		{name: "exact contract", mutate: func(content string) string { return content }},
		{name: "wrong revision", mutate: func(content string) string {
			return strings.Replace(content, releaseWorkflowRevision, strings.Repeat("0", 40), 1)
		}, wantErr: true},
		{name: "wrong module", mutate: func(content string) string {
			return strings.Replace(content, modulePath, modulePath+"-wrong", 1)
		}, wantErr: true},
		{name: "repository wide write permission", mutate: func(content string) string {
			return strings.Replace(content, "permissions: {}", "permissions:\n  contents: write", 1)
		}, wantErr: true},
		{name: "missing job permission", mutate: func(content string) string {
			return strings.Replace(content, "    permissions:\n      contents: write\n", "", 1)
		}, wantErr: true},
		{name: "missing contents permission", mutate: func(content string) string {
			return strings.Replace(content, "      contents: write\n", "", 1)
		}, wantErr: true},
		{name: "missing OIDC permission", mutate: func(content string) string {
			return strings.Replace(content, "      id-token: write\n", "", 1)
		}, wantErr: true},
		{name: "missing attestation permission", mutate: func(content string) string {
			return strings.Replace(content, "      attestations: write\n", "", 1)
		}, wantErr: true},
		{name: "missing artifact metadata permission", mutate: func(content string) string {
			return strings.Replace(content, "      artifact-metadata: write\n", "", 1)
		}, wantErr: true},
		{name: "extra package permission", mutate: func(content string) string {
			return strings.Replace(content, "      artifact-metadata: write\n", "      artifact-metadata: write\n      packages: write\n", 1)
		}, wantErr: true},
		{name: "inherited secrets", mutate: func(content string) string {
			return content + "    secrets: inherit\n"
		}, wantErr: true},
		{name: "named secret", mutate: func(content string) string {
			return content + "    secrets:\n      TOKEN: ${{ secrets.TOKEN }}\n"
		}, wantErr: true},
		{name: "wrong workflow commit input", mutate: func(content string) string {
			return strings.Replace(content, "      workflow_commit: "+releaseWorkflowRevision, "      workflow_commit: "+strings.Repeat("0", 40), 1)
		}, wantErr: true},
		{name: "distribution workflow", mutate: func(content string) string {
			return strings.Replace(content, "go-module-release.yml", "go-distribution-release.yml", 1)
		}, wantErr: true},
		{name: "extra job", mutate: func(content string) string {
			return content + "\n  bypass:\n    runs-on: ubuntu-latest\n"
		}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, ".github", "workflows", "release.yml")
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.mutate(expectedReleaseWorkflow(modulePath))), 0o600); err != nil {
				t.Fatal(err)
			}
			err := checkReleaseWorkflow(root)
			if (err != nil) != test.wantErr {
				t.Fatalf("checkReleaseWorkflow() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestCheckReleaseIntent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "exact intent", content: expectedReleaseIntent()},
		{name: "wrong version", content: strings.Replace(expectedReleaseIntent(), releaseVersion, "v0.1.0-preview.1", 1), wantErr: true},
		{name: "wrong schema", content: strings.Replace(expectedReleaseIntent(), "\"schema\": 1", "\"schema\": 2", 1), wantErr: true},
		{name: "wrong profile", content: strings.Replace(expectedReleaseIntent(), "go-module-v1", "go-distribution-v1", 1), wantErr: true},
		{name: "wrong repository", content: strings.Replace(expectedReleaseIntent(), "\"repository\": \"spice\"", "\"repository\": \"other\"", 1), wantErr: true},
		{name: "wrong module", content: strings.Replace(expectedReleaseIntent(), modulePath, modulePath+"-wrong", 1), wantErr: true},
		{name: "unknown field", content: strings.Replace(expectedReleaseIntent(), "\n}", ",\n  \"extra\": true\n}", 1), wantErr: true},
		{name: "trailing JSON", content: expectedReleaseIntent() + "{}\n", wantErr: true},
		{name: "noncanonical spacing", content: strings.Replace(expectedReleaseIntent(), "  \"schema\"", " \"schema\"", 1), wantErr: true},
		{name: "missing final newline", content: strings.TrimSuffix(expectedReleaseIntent(), "\n"), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "spice-release.json")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			err := checkReleaseIntent(root)
			if (err != nil) != test.wantErr {
				t.Fatalf("checkReleaseIntent() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
	t.Run("missing intent", func(t *testing.T) {
		t.Parallel()
		if err := checkReleaseIntent(t.TempDir()); err == nil {
			t.Fatal("checkReleaseIntent() error = nil")
		}
	})
	t.Run("intent is directory", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "spice-release.json"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := checkReleaseIntent(root); err == nil {
			t.Fatal("checkReleaseIntent() error = nil")
		}
	})
}
