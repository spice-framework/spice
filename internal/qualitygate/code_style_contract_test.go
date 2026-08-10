package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeStyleContractMatchesCanonicalRepository(t *testing.T) {
	t.Parallel()
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := (codeStyleContract{root: root}).check(); err != nil {
		t.Fatalf("code style contract: %v", err)
	}
}

func TestCodeStyleContractRejectsInventoryDrift(t *testing.T) {
	t.Parallel()
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "CODE_STYLE.md"))
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(content), "| `@Component`", "| `@MissingComponent`", 1)
	fixture := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "CODE_STYLE.md"), []byte(mutated), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyDescriptorSources(root, fixture); err != nil {
		t.Fatal(err)
	}
	if err := (codeStyleContract{root: fixture}).check(); err == nil ||
		!strings.Contains(err.Error(), "omits official descriptor Component") {
		t.Fatalf("code style drift error = %v", err)
	}
}

func copyDescriptorSources(sourceRoot, destinationRoot string) error {
	source := filepath.Join(sourceRoot, "annotation")
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(destinationRoot, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, content, 0o600)
	})
}
