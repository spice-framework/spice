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

func TestCodeStyleContractRejectsMissingVariableExceptionContract(t *testing.T) {
	t.Parallel()
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "CODE_STYLE.md"))
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(content), "\"packageVariableExceptions\": [", "\"removedVariableExceptions\": [", 1)
	fixture := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "CODE_STYLE.md"), []byte(mutated), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyDescriptorSources(root, fixture); err != nil {
		t.Fatal(err)
	}
	if err := (codeStyleContract{root: fixture}).check(); err == nil ||
		!strings.Contains(err.Error(), "packageVariableExceptions") {
		t.Fatalf("code style variable-exception drift error = %v", err)
	}
}

func TestCodeStyleContractRejectsUnreviewedCanonicalDrift(t *testing.T) {
	t.Parallel()
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "CODE_STYLE.md"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	mutated := append(append([]byte(nil), content...), []byte("unreviewed drift\n")...)
	if err := os.WriteFile(filepath.Join(fixture, "CODE_STYLE.md"), mutated, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyDescriptorSources(root, fixture); err != nil {
		t.Fatal(err)
	}
	if err := (codeStyleContract{root: fixture}).check(); err == nil ||
		!strings.Contains(err.Error(), "reviewed canonical") {
		t.Fatalf("code style canonical drift error = %v", err)
	}
}

func TestCodeStyleContractRejectsUnknownConfigurationField(t *testing.T) {
	t.Parallel()
	err := checkMutatedCodeStyleContract(t, func(content string) string {
		return strings.Replace(content, "\"profile\": \"java-structured\"", "\"unknownProfile\": \"java-structured\"", 1)
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field \"unknownProfile\"") {
		t.Fatalf("unknown configuration error = %v", err)
	}
}

func TestCodeStyleContractRejectsSchemaOneConfiguration(t *testing.T) {
	t.Parallel()
	err := checkMutatedCodeStyleContract(t, func(content string) string {
		return strings.Replace(content, "\"schemaVersion\": 2", "\"schemaVersion\": 1", 1)
	})
	if err == nil || !strings.Contains(err.Error(), "schemaVersion is 1, want 2") {
		t.Fatalf("schema version error = %v", err)
	}
}

func TestCodeStyleContractRejectsUnimplementedConfiguredRule(t *testing.T) {
	t.Parallel()
	err := checkMutatedCodeStyleContract(t, func(content string) string {
		return strings.Replace(
			content,
			"| `explicitConstructors` | typed |",
			"| `explicitConstructors` | unimplemented |",
			1,
		)
	})
	if err == nil || !strings.Contains(err.Error(), "spice.style.configuration.unsupported-rule") {
		t.Fatalf("unimplemented rule error = %v", err)
	}
}

func TestCodeStyleContractRejectsUnsupportedBuildSelection(t *testing.T) {
	t.Parallel()
	err := checkMutatedCodeStyleContract(t, func(content string) string {
		return strings.Replace(content, "\"goos\": \"linux\"", "\"goos\": \"ambient\"", 1)
	})
	if err == nil || !strings.Contains(err.Error(), "uses unsupported pair ambient/amd64") {
		t.Fatalf("unsupported build selection error = %v", err)
	}
}

func TestCodeStyleContractRejectsAmbientCGOBuildTag(t *testing.T) {
	t.Parallel()
	err := checkMutatedCodeStyleContract(t, func(content string) string {
		return strings.Replace(content, `"tags": []`, `"tags": ["cgo"]`, 1)
	})
	if err == nil || !strings.Contains(err.Error(), `has invalid tag "cgo"`) {
		t.Fatalf("ambient cgo tag error = %v", err)
	}
}

func TestCodeStyleContractRejectsBuildSelectionOrderDrift(t *testing.T) {
	t.Parallel()
	err := checkMutatedCodeStyleContract(t, func(content string) string {
		return strings.Replace(content, `"goarch": "amd64"`, `"goarch": "arm64"`, 1)
	})
	if err == nil || !strings.Contains(err.Error(), "out of canonical order") {
		t.Fatalf("build selection order error = %v", err)
	}
}

func TestCodeStyleContractRejectsUncoveredSourceRoot(t *testing.T) {
	t.Parallel()
	err := checkMutatedCodeStyleContract(t, func(content string) string {
		const roots = `      "sourceRoots": [
        "cmd",
        "internal"
      ],`
		if strings.Count(content, roots) != 4 {
			t.Fatalf("build-selection source-root blocks = %d, want 4", strings.Count(content, roots))
		}
		return strings.ReplaceAll(content, roots, `      "sourceRoots": [
        "cmd"
      ],`)
	})
	if err == nil || !strings.Contains(err.Error(), "source root \"internal\" is not covered") {
		t.Fatalf("uncovered source root error = %v", err)
	}
}

func TestCodeStyleContractRejectsDiagnosticOrderDrift(t *testing.T) {
	t.Parallel()
	err := checkMutatedCodeStyleContract(t, func(content string) string {
		const first = "| `spice.style.configuration.schema` | Invalid JSON, unknown field, or unsupported schema version |"
		const second = "| `spice.style.configuration.unsupported-rule` | Enabled rule lacks a required implementation phase |"
		content = strings.Replace(content, first, "STYLE-DIAGNOSTIC-SWAP", 1)
		content = strings.Replace(content, second, first, 1)
		return strings.Replace(content, "STYLE-DIAGNOSTIC-SWAP", second, 1)
	})
	if err == nil || !strings.Contains(err.Error(), "diagnostic table differs from the canonical ordered namespace") {
		t.Fatalf("diagnostic order error = %v", err)
	}
}

func TestStyleConfigurationLineLimitBoundaries(t *testing.T) {
	t.Parallel()
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "CODE_STYLE.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, maximum := range []string{"1", "10000"} {
		t.Run(maximum, func(t *testing.T) {
			t.Parallel()
			mutated := strings.Replace(string(content), `"maxTypeFileLines": 500`, `"maxTypeFileLines": `+maximum, 1)
			configuration, decodeErr := decodeStyleConfiguration([]byte(mutated))
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if validationErr := validateStyleConfiguration(configuration); validationErr != nil {
				t.Fatalf("line limit %s: %v", maximum, validationErr)
			}
		})
	}
}

func TestCodeStyleContractRejectsLineLimitOutsideBoundaries(t *testing.T) {
	t.Parallel()
	for _, maximum := range []string{"0", "10001"} {
		t.Run(maximum, func(t *testing.T) {
			t.Parallel()
			err := checkMutatedCodeStyleContract(t, func(content string) string {
				return strings.Replace(content, `"maxTypeFileLines": 500`, `"maxTypeFileLines": `+maximum, 1)
			})
			if err == nil || !strings.Contains(err.Error(), "must be an integer from 1 through 10000") {
				t.Fatalf("line limit %s error = %v", maximum, err)
			}
		})
	}
}

func TestCodeStyleContractRejectsSuppliedPolicyProvenanceDrift(t *testing.T) {
	t.Parallel()
	err := checkMutatedCodeStyleContract(t, func(content string) string {
		return strings.Replace(
			content,
			expectedSuppliedCodeStyleSHA256,
			strings.Repeat("0", len(expectedSuppliedCodeStyleSHA256)),
			1,
		)
	})
	if err == nil || !strings.Contains(err.Error(), "Supplied policy provenance") {
		t.Fatalf("supplied policy provenance error = %v", err)
	}
}

func checkMutatedCodeStyleContract(t *testing.T, mutate func(string) string) error {
	t.Helper()
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "CODE_STYLE.md"))
	if err != nil {
		t.Fatal(err)
	}
	mutated := mutate(string(content))
	if mutated == string(content) {
		t.Fatal("style contract mutation did not change CODE_STYLE.md")
	}
	fixture := t.TempDir()
	if err = os.WriteFile(filepath.Join(fixture, "CODE_STYLE.md"), []byte(mutated), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = copyDescriptorSources(root, fixture); err != nil {
		t.Fatal(err)
	}
	return (codeStyleContract{root: fixture}).check()
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
