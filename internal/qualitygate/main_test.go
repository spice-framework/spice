package main

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetworkAllowedOnlyForToolsBootstrap(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"fast", "check", "fmt", "verify", "verify-release", "unknown"} {
		if networkAllowed(mode) {
			t.Fatalf("networkAllowed(%q) = true", mode)
		}
	}
	if !networkAllowed("tools-bootstrap") {
		t.Fatal("networkAllowed(tools-bootstrap) = false")
	}
}

func TestBootstrapEnvironmentUsesOnlyPublicAuthenticatedGraphSettings(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"GOAUTH":      "off",
		"GOENV":       "off",
		"GOFLAGS":     "",
		"GONOPROXY":   "",
		"GONOSUMDB":   "",
		"GOPRIVATE":   "",
		"GOPROXY":     "https://proxy.golang.org",
		"GOSUMDB":     "sum.golang.org",
		"GOTOOLCHAIN": "local",
		"GOWORK":      "off",
	}
	got := bootstrapEnvironment()
	if len(got) != len(want) {
		t.Fatalf("bootstrapEnvironment() = %#v", got)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("bootstrapEnvironment()[%q] = %q, want %q", key, got[key], value)
		}
	}
}

func TestCheckBootstrapContractRequiresExactOrderedOfflineBoundary(t *testing.T) {
	t.Parallel()
	makefile := "verify:\n\tgo run ./internal/qualitygate -mode=verify\n\ntools-bootstrap:\n\tgo run ./internal/qualitygate -mode=tools-bootstrap\n"
	bootstrapStep := "      - name: Bootstrap pinned tools\n" +
		"        run: go run ./internal/qualitygate -mode=tools-bootstrap\n"
	offlineVerifyStep := "      - name: Verify offline\n" +
		"        env:\n" +
		"          GOPROXY: \"off\"\n" +
		"          GOSUMDB: \"off\"\n" +
		"          GOTOOLCHAIN: local\n" +
		"          GOWORK: \"off\"\n" +
		"        run: go run ./internal/qualitygate -mode=verify\n"
	workflow := "steps:\n" + bootstrapStep + offlineVerifyStep
	tests := []struct {
		name     string
		makefile string
		workflow string
		wantErr  bool
	}{
		{name: "exact contract", makefile: makefile, workflow: workflow},
		{name: "missing make target", makefile: strings.Replace(makefile, "\ntools-bootstrap:\n\tgo run ./internal/qualitygate -mode=tools-bootstrap\n", "", 1), workflow: workflow, wantErr: true},
		{name: "verify precedes bootstrap", makefile: makefile, workflow: "steps:\n" + offlineVerifyStep + bootstrapStep, wantErr: true},
		{name: "verification permits network", makefile: makefile, workflow: strings.Replace(workflow, "          GOPROXY: \"off\"", "          GOPROXY: https://proxy.golang.org", 1), wantErr: true},
		{name: "duplicate bootstrap", makefile: makefile, workflow: workflow + bootstrapStep, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeQualityGateFile(t, root, "Makefile", test.makefile)
			writeQualityGateFile(t, root, ".github/workflows/ci.yml", test.workflow)
			err := checkBootstrapContract(root)
			if (err != nil) != test.wantErr {
				t.Fatalf("checkBootstrapContract() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestBootstrapToolsUsesTemporaryToolsGraphAndPreservesRepository(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		runnerErr error
	}{
		{name: "success"},
		{name: "failure", runnerErr: errors.New("download failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root, toolsModule, toolsSum := toolsBootstrapFixture(t)
			before, err := sourceTreeDigests(root)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			temporaryModule := ""
			runner := func(_ context.Context, directory string, arguments ...string) error {
				calls++
				if directory != root {
					t.Fatalf("bootstrap directory = %q, want %q", directory, root)
				}
				if len(arguments) != 4 || arguments[0] != "mod" || arguments[1] != "download" || arguments[3] != "all" || !strings.HasPrefix(arguments[2], "-modfile=") {
					t.Fatalf("bootstrap arguments = %q", arguments)
				}
				temporaryModule = strings.TrimPrefix(arguments[2], "-modfile=")
				if filepath.Base(temporaryModule) != "tools.mod" || strings.HasPrefix(temporaryModule, root) {
					t.Fatalf("temporary module path = %q", temporaryModule)
				}
				gotModule, readErr := os.ReadFile(temporaryModule)
				if readErr != nil || string(gotModule) != toolsModule {
					t.Fatalf("temporary tools.mod = %q, %v", gotModule, readErr)
				}
				gotSum, readErr := os.ReadFile(strings.TrimSuffix(temporaryModule, ".mod") + ".sum")
				if readErr != nil || string(gotSum) != toolsSum {
					t.Fatalf("temporary tools.sum = %q, %v", gotSum, readErr)
				}
				return test.runnerErr
			}
			err = bootstrapTools(context.Background(), root, runner)
			if !errors.Is(err, test.runnerErr) {
				t.Fatalf("bootstrapTools() error = %v, want %v", err, test.runnerErr)
			}
			if calls != 1 {
				t.Fatalf("bootstrap calls = %d, want 1", calls)
			}
			if _, statErr := os.Stat(temporaryModule); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("temporary tools.mod remains: %v", statErr)
			}
			after, digestErr := sourceTreeDigests(root)
			if digestErr != nil || !mapsEqualDigests(before, after) {
				t.Fatalf("repository changed after bootstrap: %v", digestErr)
			}
			if _, statErr := os.Stat(filepath.Join(root, "go.sum")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("root go.sum exists after bootstrap: %v", statErr)
			}
		})
	}
}

func TestBootstrapToolsPropagatesCancellationWithoutMutation(t *testing.T) {
	t.Parallel()
	root, _, _ := toolsBootstrapFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := bootstrapTools(ctx, root, func(callContext context.Context, _ string, _ ...string) error {
		calls++
		return callContext.Err()
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("bootstrapTools() = calls %d, error %v", calls, err)
	}
}

func TestBootstrapToolsRejectsRootGraphAndDetectsMutation(t *testing.T) {
	t.Parallel()
	t.Run("preexisting root sum", func(t *testing.T) {
		t.Parallel()
		root, _, _ := toolsBootstrapFixture(t)
		writeQualityGateFile(t, root, "go.sum", "unexpected\n")
		calls := 0
		err := bootstrapTools(context.Background(), root, func(context.Context, string, ...string) error {
			calls++
			return nil
		})
		if err == nil || calls != 0 || !strings.Contains(err.Error(), "must not contain go.sum") {
			t.Fatalf("bootstrapTools() = calls %d, error %v", calls, err)
		}
	})
	t.Run("runner mutation", func(t *testing.T) {
		t.Parallel()
		root, _, _ := toolsBootstrapFixture(t)
		err := bootstrapTools(context.Background(), root, func(_ context.Context, _ string, _ ...string) error {
			return os.WriteFile(filepath.Join(root, "go.sum"), []byte("unexpected\n"), 0o600)
		})
		if err == nil || !strings.Contains(err.Error(), "modified the repository") {
			t.Fatalf("bootstrapTools() error = %v", err)
		}
	})
}

func toolsBootstrapFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	writeQualityGateFile(t, root, "go.mod", "module "+modulePath+"\n\ngo 1.26.0\n")
	toolsModule := "module " + modulePath + "/tools\n\ngo 1.26.0\n\ntool example.com/tool\n"
	toolsSum := "example.com/tool v1.0.0 h1:fixture\n"
	writeQualityGateFile(t, root, "tools/go.mod", toolsModule)
	writeQualityGateFile(t, root, "tools/go.sum", toolsSum)
	writeQualityGateFile(t, root, "marker.txt", "unchanged\n")
	return root, toolsModule, toolsSum
}

func writeQualityGateFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mapsEqualDigests(left, right map[string][32]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for name, digest := range left {
		if right[name] != digest {
			return false
		}
	}
	return true
}

func TestBestMaturityPrefixUsesMostSpecificRule(t *testing.T) {
	t.Parallel()
	rules := []maturityClassification{
		{Prefix: "annotation"},
		{Prefix: "annotation/sdk"},
	}
	got, ok := bestMaturityPrefix("annotation/sdk/protocol", rules)
	if !ok || got != "annotation/sdk" {
		t.Fatalf("bestMaturityPrefix() = %q, %v", got, ok)
	}
}

func TestAggregateCoverageWeightsStatements(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "coverage.out")
	content := "mode: atomic\n" +
		modulePath + "/bean/bean.go:1.1,2.1 3 1\n" +
		modulePath + "/web/web.go:1.1,2.1 1 0\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := aggregateCoverage(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != 75 {
		t.Fatalf("aggregateCoverage() = %v, want 75", got)
	}
}

func TestValidateMaturityClassificationsRejectsUnusedRule(t *testing.T) {
	t.Parallel()
	rules := []maturityClassification{
		{Prefix: "bean", Maturity: "preview-stable", Reason: "stable contract"},
		{Prefix: "web", Maturity: "experimental", Reason: "evolving contract"},
	}
	err := validateMaturityClassifications(rules, []string{modulePath + "/bean"})
	if err == nil {
		t.Fatal("validateMaturityClassifications() succeeded with an unused rule")
	}
}

func TestValidateImportsEnforcesToolchainBoundary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tests := []struct {
		name      string
		source    string
		wantError string
	}{
		{
			name: "standalone toolchain root import",
			source: `package bean

import _ "github.com/spice-framework/toolchain"
`,
			wantError: officialToolchainPath,
		},
		{
			name: "standalone toolchain package import",
			source: `package bean

import _ "github.com/spice-framework/toolchain/compiler/service"
`,
			wantError: officialToolchainPath + "/compiler/service",
		},
		{
			name: "descriptor tool path metadata",
			source: `package coretool

const Path = "github.com/spice-framework/toolchain/cmd/spice-annotation-core"
`,
		},
		{
			name: "standard library import",
			source: `package bean

import "context"
`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, "bean", "service.go")
			parsed, err := parser.ParseFile(
				token.NewFileSet(),
				path,
				test.source,
				parser.ImportsOnly,
			)
			if err != nil {
				t.Fatal(err)
			}
			err = validateImports(root, path, parsed)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("validateImports() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("validateImports() error = %v, want text %q", err, test.wantError)
			}
		})
	}
}
