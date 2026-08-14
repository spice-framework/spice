// Command qualitygate owns the cross-platform verification contract for the
// public Spice core module.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const (
	modulePath                 = "github.com/spice-framework/spice"
	officialToolchainPath      = "github.com/spice-framework/toolchain"
	expectedGoVersion          = "go1.26.6"
	expectedPublicPackageCount = 54
	minimumCoverage            = 85.0
)

type maturityPolicy struct {
	Schema          string                   `json:"schema"`
	Module          string                   `json:"module"`
	Classifications []maturityClassification `json:"classifications"`
}

type maturityClassification struct {
	Prefix   string `json:"prefix"`
	Maturity string `json:"maturity"`
	Reason   string `json:"reason"`
}

func main() {
	mode := flag.String("mode", "verify", "verification mode")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, *mode)
	stop()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "qualitygate: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, mode string) error {
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	if networkAllowed(mode) {
		if err := checkGoVersion(); err != nil {
			return err
		}
		if err := checkBootstrapContract(root); err != nil {
			return err
		}
		return bootstrapTools(ctx, root, networkGo)
	}
	return runOfflineMode(ctx, root, mode)
}

func runOfflineMode(ctx context.Context, root, mode string) error {
	switch mode {
	case "fmt":
		return format(ctx, root, true)
	case "fast":
		return fast(ctx, root)
	case "check":
		return check(ctx, root)
	case "vet":
		return runGo(ctx, root, nil, "vet", "./...")
	case "lint":
		return lint(ctx, root)
	case "security":
		return security(ctx, root)
	case "fuzz":
		return fuzz(ctx, root)
	case "test", "coverage":
		return testAndCoverage(ctx, root)
	case "offline":
		return offline(ctx, root)
	case "verify":
		return verify(ctx, root)
	case "verify-release":
		return verify(ctx, root)
	default:
		return fmt.Errorf("unknown mode %q", mode)
	}
}

func networkAllowed(mode string) bool {
	return mode == "tools-bootstrap"
}

func verify(ctx context.Context, root string) error {
	steps := []struct {
		name string
		run  func() error
	}{
		{"check", func() error { return check(ctx, root) }},
		{"lint and nil safety", func() error { return lint(ctx, root) }},
		{"security", func() error { return security(ctx, root) }},
		{"bounded parser and decoder fuzz smoke", func() error { return fuzz(ctx, root) }},
		{"shuffled race tests and coverage", func() error { return testAndCoverage(ctx, root) }},
		{"offline tests", func() error { return offline(ctx, root) }},
	}
	for _, step := range steps {
		if _, err := fmt.Fprintf(os.Stdout, "==> %s\n", step.name); err != nil {
			return fmt.Errorf("write verification status: %w", err)
		}
		if err := step.run(); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}
	return nil
}

func check(ctx context.Context, root string) error {
	if err := checkGoVersion(); err != nil {
		return err
	}
	if err := checkRepositoryContract(ctx, root); err != nil {
		return err
	}
	if err := checkAPIMaturity(ctx, root); err != nil {
		return err
	}
	if err := (codeStyleContract{root: root}).check(); err != nil {
		return err
	}
	if err := checkSpringCoverage(root); err != nil {
		return err
	}
	if err := format(ctx, root, false); err != nil {
		return err
	}
	if err := checkModules(ctx, root); err != nil {
		return err
	}
	return runGo(ctx, root, nil, "vet", "./...")
}

func repositoryRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(directory, "go.mod")); statErr == nil {
			return directory, nil
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return "", fmt.Errorf("inspect go.mod: %w", statErr)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("repository go.mod not found")
		}
		directory = parent
	}
}

func checkGoVersion() error {
	if runtime.Version() != expectedGoVersion {
		return fmt.Errorf("go version is %s, want %s", runtime.Version(), expectedGoVersion)
	}
	return nil
}

func publicPackages(ctx context.Context, root string) ([]string, error) {
	stdout, err := captureGo(ctx, root, offlineEnvironment(), "list", "-mod=readonly", "-f", "{{.ImportPath}}", "./...")
	if err != nil {
		return nil, err
	}
	packages := make([]string, 0, expectedPublicPackageCount)
	for line := range strings.Lines(stdout) {
		path := strings.TrimSpace(line)
		if path == "" || path == modulePath+"/internal/qualitygate" {
			continue
		}
		if !strings.HasPrefix(path, modulePath+"/") {
			return nil, fmt.Errorf("unexpected package outside core module: %s", path)
		}
		packages = append(packages, path)
	}
	sort.Strings(packages)
	if len(packages) != expectedPublicPackageCount {
		return nil, fmt.Errorf("public package count is %d, want %d", len(packages), expectedPublicPackageCount)
	}
	return packages, nil
}

func checkRepositoryContract(ctx context.Context, root string) error {
	for _, check := range []func(string) error{
		checkFastTarget,
		checkBootstrapContract,
		checkReleaseContract,
	} {
		if err := check(root); err != nil {
			return err
		}
	}
	files, err := repositoryFiles(ctx, root)
	if err != nil {
		return err
	}
	forbidden := []string{".spice/", "benchmarks/", "cmd/", "compiler/", "testdata/", "vendor/"}
	retired := []string{
		"github.com/" + "StevenBuglione/spice",
		modulePath + "/cmd/spice",
		"go run " + "./cmd/spice",
	}
	for _, path := range files {
		slash := filepath.ToSlash(path)
		for _, prefix := range forbidden {
			if strings.HasPrefix(slash, prefix) {
				return fmt.Errorf("toolchain-owned path remains in core: %s", slash)
			}
		}
		if strings.HasPrefix(slash, "internal/") && !strings.HasPrefix(slash, "internal/qualitygate/") {
			return fmt.Errorf("unsupported internal implementation remains in core: %s", slash)
		}
		if !isTextContractFile(slash) || strings.HasPrefix(slash, ".tmp/") {
			continue
		}
		// #nosec G304 -- path is constrained to the repository file list.
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(slash)))
		if readErr != nil {
			return fmt.Errorf("read %s: %w", slash, readErr)
		}
		for _, identity := range retired {
			if bytes.Contains(content, []byte(identity)) {
				return fmt.Errorf("retired toolchain identity %q remains in %s", identity, slash)
			}
		}
	}
	if err := checkCoretoolPath(root); err != nil {
		return err
	}
	return checkImportDirections(root)
}

func checkBootstrapContract(root string) error {
	makefile, err := os.ReadFile(filepath.Join(root, "Makefile")) // #nosec G304 -- root and Makefile path are repository-owned.
	if err != nil {
		return fmt.Errorf("read Makefile bootstrap target: %w", err)
	}
	normalizedMakefile := strings.ReplaceAll(string(makefile), "\r\n", "\n")
	wantTarget := "\ntools-bootstrap:\n\tgo run ./internal/qualitygate -mode=tools-bootstrap\n"
	if strings.Count(normalizedMakefile, wantTarget) != 1 {
		return errors.New("tools-bootstrap target must invoke the explicit dependency bootstrap exactly once")
	}

	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml")) // #nosec G304 -- root and CI workflow path are repository-owned.
	if err != nil {
		return fmt.Errorf("read CI bootstrap contract: %w", err)
	}
	normalizedWorkflow := strings.ReplaceAll(string(workflow), "\r\n", "\n")
	bootstrap := "      - name: Bootstrap pinned tools\n        run: go run ./internal/qualitygate -mode=tools-bootstrap\n"
	offlineVerify := "      - name: Verify offline\n        env:\n          GOPROXY: \"off\"\n          GOSUMDB: \"off\"\n          GOTOOLCHAIN: local\n          GOWORK: \"off\"\n        run: go run ./internal/qualitygate -mode=verify\n"
	bootstrapIndex := strings.Index(normalizedWorkflow, bootstrap)
	verifyIndex := strings.Index(normalizedWorkflow, offlineVerify)
	if bootstrapIndex < 0 || verifyIndex <= bootstrapIndex ||
		strings.Count(normalizedWorkflow, bootstrap) != 1 || strings.Count(normalizedWorkflow, offlineVerify) != 1 {
		return errors.New("CI must bootstrap the exact pinned tools once before the exact offline verification step")
	}
	return nil
}

type bootstrapRunner func(context.Context, string, ...string) error

func bootstrapTools(ctx context.Context, root string, runner bootstrapRunner) (returnErr error) {
	before, err := sourceTreeDigests(root)
	if err != nil {
		return fmt.Errorf("snapshot repository before tools bootstrap: %w", err)
	}
	defer func() {
		after, snapshotErr := sourceTreeDigests(root)
		if snapshotErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("snapshot repository after tools bootstrap: %w", snapshotErr))
			return
		}
		if !maps.Equal(before, after) {
			returnErr = errors.Join(returnErr, errors.New("tools bootstrap modified the repository"))
		}
	}()

	rootSum := filepath.Join(root, "go.sum")
	if _, statErr := os.Lstat(rootSum); statErr == nil {
		return errors.New("standard-library-only root graph must not contain go.sum")
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("inspect root go.sum: %w", statErr)
	}
	toolsDirectory := filepath.Join(root, "tools")
	moduleContent, err := os.ReadFile(filepath.Join(toolsDirectory, "go.mod")) // #nosec G304 -- tools module path is repository-owned.
	if err != nil {
		return fmt.Errorf("read tools/go.mod: %w", err)
	}
	sumContent, err := os.ReadFile(filepath.Join(toolsDirectory, "go.sum")) // #nosec G304 -- tools checksum path is repository-owned.
	if err != nil {
		return fmt.Errorf("read tools/go.sum: %w", err)
	}

	temporary, err := os.MkdirTemp("", "spice-core-tools-bootstrap-*")
	if err != nil {
		return fmt.Errorf("create tools bootstrap directory: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, removeTemporaryDirectory(temporary))
	}()
	temporaryRoot, err := os.OpenRoot(temporary)
	if err != nil {
		return fmt.Errorf("open tools bootstrap directory: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, temporaryRoot.Close())
	}()
	if err := temporaryRoot.WriteFile("tools.mod", moduleContent, 0o600); err != nil {
		return fmt.Errorf("write temporary tools.mod: %w", err)
	}
	if err := temporaryRoot.WriteFile("tools.sum", sumContent, 0o600); err != nil {
		return fmt.Errorf("write temporary tools.sum: %w", err)
	}
	return runner(ctx, root, bootstrapDownloadArguments(filepath.Join(temporary, "tools.mod"))...)
}

func bootstrapDownloadArguments(moduleFile string) []string {
	return []string{"mod", "download", "-modfile=" + moduleFile, "all"}
}

func sourceTreeDigests(root string) (_ map[string][sha256.Size]byte, returnErr error) {
	opened, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open repository tree: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, opened.Close())
	}()
	digests := make(map[string][sha256.Size]byte)
	err = fs.WalkDir(opened.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && (path == ".git" || path == ".tmp") {
			return fs.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		content, readErr := opened.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read repository file %s: %w", filepath.ToSlash(path), readErr)
		}
		digests[filepath.ToSlash(path)] = sha256.Sum256(content)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("digest repository tree: %w", err)
	}
	return digests, nil
}

func repositoryFiles(ctx context.Context, root string) ([]string, error) {
	stdout, err := capture(ctx, root, nil, "git", "ls-files", "-co", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	files := make([]string, 0)
	for line := range strings.Lines(stdout) {
		path := strings.TrimSpace(line)
		if path == "" || strings.HasPrefix(filepath.ToSlash(path), ".tmp/") {
			continue
		}
		if info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(path))); statErr == nil && !info.IsDir() {
			files = append(files, filepath.ToSlash(path))
		} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("inspect %s: %w", path, statErr)
		}
	}
	sort.Strings(files)
	return files, nil
}

func isTextContractFile(path string) bool {
	if path == "Makefile" || path == "LICENSE" {
		return true
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".json", ".md", ".mod", ".sum", ".txt", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func checkCoretoolPath(root string) error {
	path := filepath.Join(root, "annotation", "coretool", "path.go")
	// #nosec G304 -- path is a fixed repository-owned source file.
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read annotation core tool path: %w", err)
	}
	want := `const Path = "` + officialToolchainPath + `/cmd/spice-annotation-core"`
	if !bytes.Contains(content, []byte(want)) {
		return fmt.Errorf("annotation/coretool.Path must identify %s/cmd/spice-annotation-core", officialToolchainPath)
	}
	return nil
}

func checkImportDirections(root string) error {
	set := token.NewFileSet()
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return skipImplementationDirectory(root, path)
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(set, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse imports in %s: %w", path, err)
		}
		return validateImports(root, path, parsed)
	})
}

func skipImplementationDirectory(root, path string) error {
	if path == root {
		return nil
	}
	name := filepath.Base(path)
	if strings.HasPrefix(name, ".") || name == "tools" || name == "internal" {
		return filepath.SkipDir
	}
	return nil
}

func validateImports(root, path string, parsed *ast.File) error {
	for _, imported := range parsed.Imports {
		value, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return fmt.Errorf("decode import in %s: %w", path, err)
		}
		for _, prefix := range []string{
			officialToolchainPath,
			modulePath + "/cmd",
			modulePath + "/compiler",
			modulePath + "/internal",
		} {
			if value == prefix || strings.HasPrefix(value, prefix+"/") {
				relative, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return fmt.Errorf("relativize %s: %w", path, relErr)
				}
				return fmt.Errorf("public package %s imports toolchain implementation %s", filepath.ToSlash(relative), value)
			}
		}
	}
	return nil
}

func checkAPIMaturity(ctx context.Context, root string) error {
	path := filepath.Join(root, "docs", "api-compatibility.json")
	// #nosec G304 -- path is a fixed repository-owned policy file.
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		return fmt.Errorf("read API maturity policy: %w", readErr)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var policy maturityPolicy
	if decodeErr := decoder.Decode(&policy); decodeErr != nil {
		return fmt.Errorf("decode API maturity policy: %w", decodeErr)
	}
	if trailingErr := requireJSONEOF(decoder); trailingErr != nil {
		return trailingErr
	}
	if policy.Schema != "spice.api-maturity/v1" || policy.Module != modulePath {
		return errors.New("API maturity policy has an invalid schema or module")
	}
	packages, err := publicPackages(ctx, root)
	if err != nil {
		return err
	}
	return validateMaturityClassifications(policy.Classifications, packages)
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode trailing API policy content: %w", err)
	}
	return errors.New("API maturity policy contains multiple JSON values")
}

func validateMaturityClassifications(rules []maturityClassification, packages []string) error {
	used := make(map[string]bool, len(rules))
	seen := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		if rule.Prefix == "" || strings.TrimSpace(rule.Reason) == "" {
			return errors.New("API maturity classification requires prefix and reason")
		}
		if _, duplicate := seen[rule.Prefix]; duplicate {
			return fmt.Errorf("duplicate API maturity prefix %q", rule.Prefix)
		}
		seen[rule.Prefix] = struct{}{}
		switch rule.Maturity {
		case "internal", "experimental", "preview-stable":
		default:
			return fmt.Errorf("invalid API maturity %q for %s", rule.Maturity, rule.Prefix)
		}
	}
	for _, packagePath := range packages {
		relative := strings.TrimPrefix(packagePath, modulePath+"/")
		prefix, ok := bestMaturityPrefix(relative, rules)
		if !ok {
			return fmt.Errorf("public package %s has no API maturity classification", packagePath)
		}
		used[prefix] = true
	}
	for _, rule := range rules {
		if !used[rule.Prefix] {
			return fmt.Errorf("API maturity prefix %q matches no public package", rule.Prefix)
		}
	}
	return nil
}

func bestMaturityPrefix(path string, rules []maturityClassification) (string, bool) {
	best := ""
	for _, rule := range rules {
		if (path == rule.Prefix || strings.HasPrefix(path, rule.Prefix+"/")) && len(rule.Prefix) > len(best) {
			best = rule.Prefix
		}
	}
	return best, best != ""
}

func checkSpringCoverage(root string) error {
	path := filepath.Join(root, "docs", "spring-coverage.md")
	// #nosec G304 -- path is a fixed repository-owned documentation file.
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read Spring coverage map: %w", err)
	}
	rows := 0
	for line := range strings.Lines(string(content)) {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "|---") || strings.Contains(trimmed, "| Status |") {
			continue
		}
		columns := strings.Split(trimmed, "|")
		if len(columns) < 6 {
			return fmt.Errorf("malformed Spring coverage row: %s", trimmed)
		}
		status := strings.TrimSpace(columns[len(columns)-2])
		switch status {
		case "available", "integration", "not-planned":
			rows++
		default:
			return fmt.Errorf("invalid Spring coverage status %q", status)
		}
	}
	if rows == 0 {
		return errors.New("spring coverage map contains no capability rows")
	}
	return nil
}

func format(ctx context.Context, root string, write bool) error {
	files, err := goFiles(root)
	if err != nil {
		return err
	}
	option := "-l"
	if write {
		option = "-w"
	}
	for _, tool := range []string{"goimports", "gofumpt"} {
		path, resolveErr := toolPath(ctx, root, tool)
		if resolveErr != nil {
			return resolveErr
		}
		for start := 0; start < len(files); start += 100 {
			end := min(start+100, len(files))
			args := append([]string{option}, files[start:end]...)
			if write {
				if err := command(ctx, root, nil, path, args...); err != nil {
					return err
				}
				continue
			}
			stdout, err := capture(ctx, root, nil, path, args...)
			if err != nil {
				return err
			}
			if strings.TrimSpace(stdout) != "" {
				return fmt.Errorf("%s reports unformatted files:\n%s", tool, stdout)
			}
		}
	}
	return nil
}

func goFiles(root string) ([]string, error) {
	files := make([]string, 0, 256)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root {
				name := entry.Name()
				if strings.HasPrefix(name, ".") || name == "tools" || name == "vendor" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if filepath.Ext(path) == ".go" {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return fmt.Errorf("relativize Go source %s: %w", path, err)
			}
			files = append(files, relative)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk Go source: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func checkModules(ctx context.Context, root string) error {
	if err := tidyModule(ctx, root); err != nil {
		return err
	}
	if err := tidyModule(ctx, filepath.Join(root, "tools")); err != nil {
		return err
	}
	stdout, err := captureGo(ctx, root, offlineEnvironment(), "list", "-mod=readonly", "-m", "all")
	if err != nil {
		return err
	}
	if strings.TrimSpace(stdout) != modulePath {
		return fmt.Errorf("core module must be standard-library-only; module graph is:\n%s", stdout)
	}
	return reproduceEmptyVendor(ctx, root)
}

func tidyModule(ctx context.Context, directory string) error {
	stdout, err := captureGo(ctx, directory, nil, "mod", "tidy", "-diff")
	if err != nil {
		return err
	}
	if strings.TrimSpace(stdout) != "" {
		return fmt.Errorf("module is not tidy in %s:\n%s", directory, stdout)
	}
	return nil
}

func reproduceEmptyVendor(ctx context.Context, root string) (returnErr error) {
	temporary, err := os.MkdirTemp("", "spice-core-vendor-*")
	if err != nil {
		return fmt.Errorf("create vendor verification directory: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, removeTemporaryDirectory(temporary))
	}()
	destination := filepath.Join(temporary, "vendor")
	if commandErr := runGo(ctx, root, offlineEnvironment(), "mod", "vendor", "-o", destination); commandErr != nil {
		return commandErr
	}
	entries, err := os.ReadDir(destination)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect reproduced vendor directory: %w", err)
	}
	if len(entries) != 0 {
		return errors.New("stdlib-only core unexpectedly produces vendor content")
	}
	return nil
}

func lint(ctx context.Context, root string) error {
	golangci, err := toolPath(ctx, root, "golangci-lint")
	if err != nil {
		return err
	}
	if commandErr := command(ctx, root, nil, golangci, "run", "--timeout=10m"); commandErr != nil {
		return commandErr
	}
	nilaway, err := toolPath(ctx, root, "nilaway")
	if err != nil {
		return err
	}
	return command(ctx, root, nil, nilaway, "-include-pkgs="+modulePath, "./...")
}

func security(ctx context.Context, root string) error {
	gosec, err := toolPath(ctx, root, "gosec")
	if err != nil {
		return err
	}
	if commandErr := command(ctx, root, nil, gosec, "-quiet", "-exclude-generated", "./..."); commandErr != nil {
		return commandErr
	}
	govulncheck, err := toolPath(ctx, root, "govulncheck")
	if err != nil {
		return err
	}
	return command(ctx, root, nil, govulncheck, "./...")
}

func fuzz(ctx context.Context, root string) error {
	targets := []struct {
		packagePath string
		name        string
	}{
		{"./annotation/sdk/protocol", "FuzzReadMessage"},
		{"./annotation/sdk/starter", "FuzzParseManifest"},
		{"./config", "FuzzDecodeJSONObject"},
		{"./expression", "FuzzCompile"},
		{"./web", "FuzzDecodeJSON"},
	}
	for _, target := range targets {
		if err := runGo(
			ctx,
			root,
			offlineEnvironment(),
			"test",
			"-mod=readonly",
			target.packagePath,
			"-run=^$",
			"-fuzz=^"+target.name+"$",
			"-fuzztime=100x",
		); err != nil {
			return err
		}
	}
	return nil
}

func testAndCoverage(ctx context.Context, root string) (returnErr error) {
	packages, err := publicPackages(ctx, root)
	if err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "spice-core-coverage-*")
	if err != nil {
		return fmt.Errorf("create coverage directory: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, removeTemporaryDirectory(temporary))
	}()
	profile := filepath.Join(temporary, "coverage.out")
	args := []string{"test", "-mod=readonly", "-race", "-shuffle=on", "-count=1", "-covermode=atomic", "-coverprofile=" + profile}
	args = append(args, packages...)
	if commandErr := runGo(ctx, root, nil, args...); commandErr != nil {
		return commandErr
	}
	coverage, err := aggregateCoverage(profile)
	if err != nil {
		return err
	}
	if _, writeErr := fmt.Fprintf(os.Stdout, "coverage: %.1f%% (minimum %.1f%%)\n", coverage, minimumCoverage); writeErr != nil {
		return fmt.Errorf("write coverage status: %w", writeErr)
	}
	if coverage < minimumCoverage {
		return fmt.Errorf("public-source coverage %.1f%% is below %.1f%%", coverage, minimumCoverage)
	}
	return nil
}

func aggregateCoverage(path string) (float64, error) {
	// #nosec G304 -- path is a qualitygate-owned temporary profile.
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read coverage profile: %w", err)
	}
	covered, statements := 0, 0
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 || !strings.HasPrefix(fields[0], modulePath+"/") {
			return 0, fmt.Errorf("invalid public coverage row %q", line)
		}
		count, countErr := strconv.Atoi(fields[2])
		if countErr != nil {
			return 0, fmt.Errorf("decode coverage count: %w", countErr)
		}
		blocks, blocksErr := strconv.Atoi(fields[1])
		if blocksErr != nil {
			return 0, fmt.Errorf("decode coverage statements: %w", blocksErr)
		}
		statements += blocks
		if count > 0 {
			covered += blocks
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan coverage profile: %w", err)
	}
	if statements == 0 {
		return 0, errors.New("coverage profile contains no public statements")
	}
	return float64(covered) * 100 / float64(statements), nil
}

func removeTemporaryDirectory(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove temporary directory %s: %w", path, err)
	}
	return nil
}

func offline(ctx context.Context, root string) error {
	packages, err := publicPackages(ctx, root)
	if err != nil {
		return err
	}
	args := []string{"test", "-mod=vendor", "-count=1"}
	args = append(args, packages...)
	return runGo(ctx, root, offlineEnvironment(), args...)
}

func offlineEnvironment() map[string]string {
	return map[string]string{
		"GOPROXY":     "off",
		"GOSUMDB":     "off",
		"GOTOOLCHAIN": "local",
		"GOWORK":      "off",
	}
}

func toolPath(ctx context.Context, root, name string) (string, error) {
	stdout, err := captureGo(ctx, root, nil, "tool", "-C", "tools", "-n", name)
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(stdout)
	if path == "" {
		return "", fmt.Errorf("resolve tool %q: empty path", name)
	}
	return path, nil
}

func runGo(ctx context.Context, directory string, environment map[string]string, args ...string) error {
	return command(ctx, directory, environment, "go", args...)
}

func networkGo(ctx context.Context, directory string, args ...string) error {
	return runGo(ctx, directory, bootstrapEnvironment(), args...)
}

func bootstrapEnvironment() map[string]string {
	return map[string]string{
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
}

func captureGo(ctx context.Context, directory string, environment map[string]string, args ...string) (string, error) {
	return capture(ctx, directory, environment, "go", args...)
}

func command(ctx context.Context, directory string, environment map[string]string, executable string, args ...string) error {
	// #nosec G204 -- executable is either the Go binary or a path resolved by `go tool -n`; arguments are discrete and never interpreted by a shell.
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = directory
	cmd.Env = mergedEnvironment(environment)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", filepath.Base(executable), strings.Join(args, " "), err)
	}
	return nil
}

func capture(ctx context.Context, directory string, environment map[string]string, executable string, args ...string) (string, error) {
	// #nosec G204 -- executable is a fixed quality tool and arguments are discrete without a shell.
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = directory
	cmd.Env = mergedEnvironment(environment)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", filepath.Base(executable), strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func mergedEnvironment(overrides map[string]string) []string {
	if len(overrides) == 0 {
		return os.Environ()
	}
	values := make(map[string]string, len(os.Environ())+len(overrides))
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		values[environmentKey(key)] = value
	}
	for key, value := range overrides {
		values[environmentKey(key)] = key + "=" + value
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func environmentKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}
