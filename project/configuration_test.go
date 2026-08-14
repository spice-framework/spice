package project_test

import (
	"strings"
	"testing"

	"github.com/spice-framework/spice/project"
)

func TestConfigurationConstructors(t *testing.T) {
	t.Parallel()

	if included := project.Include("payments", "services/payments"); included.Name != "payments" || included.Directory != "services/payments" {
		t.Fatalf("Include() = %#v", included)
	}
	assertDependency(t, project.Starter("web"), project.DependencyStarter, project.ScopeMain, "web", "", "")
	assertDependency(t, project.Library("github.com/google/uuid", "v1.6.0"), project.DependencyLibrary, project.ScopeMain, "", "github.com/google/uuid", "v1.6.0")
	assertDependency(t, project.TestLibrary("github.com/stretchr/testify", "v1.11.1"), project.DependencyLibrary, project.ScopeTest, "", "github.com/stretchr/testify", "v1.11.1")
	assertDependency(t, project.BuildTool("example.com/tools/codegen", "v1.2.3"), project.DependencyTool, project.ScopeBuild, "", "example.com/tools/codegen", "v1.2.3")

	if plugin := project.ApplicationPlugin(); plugin.Kind != project.PluginApplication || plugin.Module != "" || plugin.Version != "" {
		t.Fatalf("ApplicationPlugin() = %#v", plugin)
	}
	if plugin := project.CompilerPlugin("example.com/compiler", "v1.2.3"); plugin.Kind != project.PluginCompiler || plugin.Module != "example.com/compiler" || plugin.Version != "v1.2.3" {
		t.Fatalf("CompilerPlugin() = %#v", plugin)
	}
	if target := project.ApplicationTarget("commerce", "example.com/commerce/cmd/commerce", "example.com/commerce/internal/spicegen/commerce"); target.Name != "commerce" || target.Package == "" || target.Generated == "" {
		t.Fatalf("ApplicationTarget() = %#v", target)
	}
	if generator := project.CodeGenerator("example.com/codegen", "v1.2.3"); generator.Module != "example.com/codegen" || generator.Version != "v1.2.3" {
		t.Fatalf("CodeGenerator() = %#v", generator)
	}
	if override := project.PlaceType("example.com/commerce/internal/users.UserMapper", "users/application"); override.Kind != project.ViewPlaceType || override.GoSymbol == "" || override.ViewGroup != "users/application" {
		t.Fatalf("PlaceType() = %#v", override)
	}
	if replacement := project.Replace("example.com/module", "../module"); replacement.Module != "example.com/module" || replacement.Directory != "../module" {
		t.Fatalf("Replace() = %#v", replacement)
	}
	if dependency := project.CatalogLibrary("example.com/library", "shared"); dependency.Module != "example.com/library" || dependency.Version != "shared" {
		t.Fatalf("CatalogLibrary() = %#v", dependency)
	}
	if dependency := project.CatalogStarter("example.com/starter-web", "v1.2.3"); dependency.Module != "example.com/starter-web" || dependency.Version != "v1.2.3" {
		t.Fatalf("CatalogStarter() = %#v", dependency)
	}
}

func TestValidateSettings(t *testing.T) {
	t.Parallel()

	settings := validSettings()
	settings.Projects = project.IncludedProjects{
		project.Include("orders", "services/orders"),
		project.Include("payments", "services/payments"),
	}
	settings.DependencyPolicy = project.DependencyPolicy{
		Verification:       project.Strict,
		ApprovedRegistries: []string{"https://modules.example.com"},
		ApprovedProxies:    []string{"https://proxy.example.com"},
		AllowedModules:     []string{"example.com/approved"},
		DeniedModules:      []string{"example.com/denied"},
	}
	if err := project.ValidateSettings(settings); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSettingsRejectsInvalidContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*project.Settings)
		want   string
	}{
		{"name", func(value *project.Settings) { value.Name = "Commerce" }, "project name"},
		{"module", func(value *project.Settings) { value.Module = "../commerce" }, "module"},
		{"unicode module", func(value *project.Settings) { value.Module = "example.com/café" }, "module"},
		{"go version", func(value *project.Settings) { value.Toolchain.Go = "go1.26.6" }, "Go version"},
		{"Spice version", func(value *project.Settings) { value.Toolchain.Spice = "0.5.0" }, "Spice version"},
		{"included name", func(value *project.Settings) {
			value.Projects = project.IncludedProjects{project.Include("Bad", "services/bad")}
		}, "included projects"},
		{"included path", func(value *project.Settings) {
			value.Projects = project.IncludedProjects{project.Include("bad", "../bad")}
		}, "directory"},
		{"duplicate included name", func(value *project.Settings) {
			value.Projects = project.IncludedProjects{project.Include("same", "a"), project.Include("same", "b")}
		}, "duplicated"},
		{"case-folded included path", func(value *project.Settings) {
			value.Projects = project.IncludedProjects{project.Include("one", "Services/one"), project.Include("two", "services/ONE")}
		}, "case folding"},
		{"verification", func(value *project.Settings) { value.DependencyPolicy.Verification = "lenient" }, "verification mode"},
		{"policy order", func(value *project.Settings) { value.DependencyPolicy.ApprovedProxies = []string{"z", "a"} }, "canonical order"},
		{"policy duplicate", func(value *project.Settings) { value.DependencyPolicy.ApprovedRegistries = []string{"same", "same"} }, "duplicate"},
		{"policy invalid module", func(value *project.Settings) { value.DependencyPolicy.AllowedModules = []string{"../bad"} }, "policy module"},
		{"policy conflict", func(value *project.Settings) {
			value.DependencyPolicy.AllowedModules = []string{"example.com/same"}
			value.DependencyPolicy.DeniedModules = []string{"example.com/same"}
		}, "both allowed and denied"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			settings := validSettings()
			test.mutate(&settings)
			assertErrorContains(t, project.ValidateSettings(settings), test.want)
		})
	}
}

func TestValidateBuild(t *testing.T) {
	t.Parallel()

	build := validBuild()
	build.Dependencies = project.Dependencies{
		project.Starter("web"),
		project.Library("github.com/google/uuid", "v1.6.0"),
		project.TestLibrary("github.com/stretchr/testify", "v1.11.1"),
		project.BuildTool("example.com/tools/generator", "v1.2.3"),
	}
	build.Plugins = append(build.Plugins, project.CompilerPlugin("example.com/compiler", "v1.2.3"))
	build.Targets = project.Targets{project.ApplicationTarget(
		"commerce",
		"example.com/commerce/cmd/commerce",
		"example.com/commerce/internal/spicegen/commerce",
	)}
	build.Generators = project.Generators{project.CodeGenerator("example.com/generator", "v1.2.3")}
	build.StyleExceptions = project.StyleExceptions{
		project.AllowPackageFunction("example.com/commerce/internal/platform", "Register", "tool boundary", "ARCH-123"),
		project.AllowPublicRoute("example.com/commerce/internal/catalog", "CatalogController", "List", "anonymous catalog", "SEC-123"),
	}
	build.Views = project.ViewOverrides{
		project.PlaceType("example.com/commerce/internal/orders.OrderMapper", "orders/application"),
		project.PlaceType("example.com/commerce/internal/users.UserMapper", "users/application"),
	}
	build.Packaging = project.Packaging{Formats: []project.PackageFormat{project.PackageArchive, project.PackageBinary}}
	if err := project.ValidateBuild(build); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBuildRejectsInvalidContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*project.Build)
		want   string
	}{
		{"kind", func(value *project.Build) { value.Kind = "unknown" }, "build kind"},
		{"starter fields", func(value *project.Build) {
			value.Dependencies = project.Dependencies{{Kind: project.DependencyStarter, Scope: project.ScopeMain, Name: "Bad"}}
		}, "starter dependency"},
		{"starter scope", func(value *project.Build) {
			value.Dependencies = project.Dependencies{{Kind: project.DependencyStarter, Scope: project.ScopeBuild, Name: "web"}}
		}, "unsupported scope"},
		{"starter version", func(value *project.Build) {
			value.Dependencies = project.Dependencies{{Kind: project.DependencyStarter, Scope: project.ScopeMain, Name: "web", Version: "latest"}}
		}, "version"},
		{"library scope", func(value *project.Build) {
			value.Dependencies = project.Dependencies{{Kind: project.DependencyLibrary, Scope: project.ScopeBuild, Module: "example.com/library", Version: "v1.2.3"}}
		}, "unsupported scope"},
		{"library fields", func(value *project.Build) {
			value.Dependencies = project.Dependencies{{Kind: project.DependencyLibrary, Scope: project.ScopeMain, Name: "named", Module: "example.com/library", Version: "v1.2.3"}}
		}, "library dependency"},
		{"tool fields", func(value *project.Build) {
			value.Dependencies = project.Dependencies{{Kind: project.DependencyTool, Scope: project.ScopeMain, Module: "example.com/tool", Version: "v1.2.3"}}
		}, "tool dependency"},
		{"dependency kind", func(value *project.Build) {
			value.Dependencies = project.Dependencies{{Kind: "unknown", Scope: project.ScopeMain}}
		}, "dependency kind"},
		{"dependency duplicate", func(value *project.Build) {
			value.Dependencies = project.Dependencies{project.Starter("web"), project.Starter("web")}
		}, "duplicated"},
		{"missing application plugin", func(value *project.Build) { value.Plugins = nil }, "exactly one application plugin"},
		{"application plugin fields", func(value *project.Build) {
			value.Plugins = project.Plugins{{Kind: project.PluginApplication, Module: "bad"}}
		}, "cannot declare"},
		{"compiler plugin", func(value *project.Build) {
			value.Plugins = append(value.Plugins, project.CompilerPlugin("bad module", "latest"))
		}, "compiler plugin"},
		{"plugin kind", func(value *project.Build) { value.Plugins = append(value.Plugins, project.Plugin{Kind: "unknown"}) }, "plugin kind"},
		{"duplicate plugin", func(value *project.Build) { value.Plugins = append(value.Plugins, project.ApplicationPlugin()) }, "duplicated"},
		{"plugin on library", func(value *project.Build) { value.Kind = project.LibraryKind }, "cannot select"},
		{"target name", func(value *project.Build) {
			value.Targets = project.Targets{project.ApplicationTarget("Bad", "example.com/cmd/app", "example.com/internal/spicegen/app")}
		}, "target name"},
		{"target package", func(value *project.Build) {
			value.Targets = project.Targets{project.ApplicationTarget("app", "../bad", "example.com/generated")}
		}, "package paths"},
		{"duplicate target", func(value *project.Build) {
			value.Targets = project.Targets{project.ApplicationTarget("app", "example.com/cmd/a", "example.com/gen/a"), project.ApplicationTarget("app", "example.com/cmd/b", "example.com/gen/b")}
		}, "duplicated"},
		{"generator", func(value *project.Build) {
			value.Generators = project.Generators{project.CodeGenerator("bad module", "latest")}
		}, "generator"},
		{"duplicate generator", func(value *project.Build) {
			value.Generators = project.Generators{project.CodeGenerator("example.com/generator", "v1.2.3"), project.CodeGenerator("example.com/generator", "v1.2.4")}
		}, "duplicated"},
		{"style common", func(value *project.Build) {
			value.StyleExceptions = project.StyleExceptions{project.AllowPackageFunction("../bad", "Register", "", "")}
		}, "requires a Go package"},
		{"style function", func(value *project.Build) {
			value.StyleExceptions = project.StyleExceptions{{Kind: project.StylePackageFunction, Package: "example.com/app", Symbol: "bad-name", Reason: "reason", Issue: "ARCH-1"}}
		}, "invalid fields"},
		{"style variable", func(value *project.Build) {
			value.StyleExceptions = project.StyleExceptions{{Kind: project.StylePackageVariable, Package: "example.com/app", Symbol: "files", Reason: "reason", Issue: "ARCH-1"}}
		}, "invalid fields"},
		{"style route", func(value *project.Build) {
			value.StyleExceptions = project.StyleExceptions{{Kind: project.StylePublicRoute, Package: "example.com/app", Receiver: "controller", Method: "List", Reason: "reason", Issue: "SEC-1"}}
		}, "invalid fields"},
		{"style kind", func(value *project.Build) {
			value.StyleExceptions = project.StyleExceptions{{Kind: "unknown", Package: "example.com/app", Reason: "reason", Issue: "ARCH-1"}}
		}, "style exception kind"},
		{"style order", func(value *project.Build) {
			value.StyleExceptions = project.StyleExceptions{project.AllowPublicRoute("example.com/app", "Controller", "List", "reason", "SEC-1"), project.AllowPackageFunction("example.com/app", "Register", "reason", "ARCH-1")}
		}, "canonical order"},
		{"View kind", func(value *project.Build) { value.Views = project.ViewOverrides{{Kind: "unknown"}} }, "view override kind"},
		{"View symbol", func(value *project.Build) {
			value.Views = project.ViewOverrides{project.PlaceType("users.user", "users/domain")}
		}, "canonical exported Go type"},
		{"View path", func(value *project.Build) {
			value.Views = project.ViewOverrides{project.PlaceType("example.com/app/internal/users.User", "../users")}
		}, "canonical relative path"},
		{"View reserved path", func(value *project.Build) {
			value.Views = project.ViewOverrides{project.PlaceType("example.com/app/internal/users.User", "users/CON")}
		}, "canonical relative path"},
		{"View order", func(value *project.Build) {
			value.Views = project.ViewOverrides{project.PlaceType("example.com/app/internal/z.Z", "z/domain"), project.PlaceType("example.com/app/internal/a.A", "a/domain")}
		}, "canonical order"},
		{"package format", func(value *project.Build) { value.Packaging.Formats = []project.PackageFormat{"image"} }, "unsupported"},
		{"package format order", func(value *project.Build) {
			value.Packaging.Formats = []project.PackageFormat{project.PackageBinary, project.PackageArchive}
		}, "canonical order"},
		{"package format duplicate", func(value *project.Build) {
			value.Packaging.Formats = []project.PackageFormat{project.PackageBinary, project.PackageBinary}
		}, "duplicate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			build := validBuild()
			test.mutate(&build)
			assertErrorContains(t, project.ValidateBuild(build), test.want)
		})
	}
}

func TestValidateCatalogAndLocal(t *testing.T) {
	t.Parallel()

	catalog := project.Catalog{
		Versions: project.Versions{"shared": "v1.2.3"},
		Libraries: project.CatalogDependencies{
			"uuid": project.CatalogLibrary("github.com/google/uuid", "shared"),
		},
		Starters: project.CatalogDependencies{
			"web": project.CatalogStarter("github.com/spice-framework/starter-web", "v1.2.3"),
		},
	}
	if err := project.ValidateCatalog(catalog); err != nil {
		t.Fatal(err)
	}
	local := project.Local{
		Replacements:      project.Replacements{project.Replace("github.com/spice-framework/spice", "../spice")},
		ToolPaths:         project.ToolPaths{"spice": "tools/spice"},
		WorkspaceProvider: project.MaterializedWorkspace,
	}
	if err := project.ValidateLocal(local); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCatalogAndLocalRejectInvalidContracts(t *testing.T) {
	t.Parallel()

	catalogTests := []struct {
		name    string
		catalog project.Catalog
		want    string
	}{
		{"version alias", project.Catalog{Versions: project.Versions{"Bad": "v1.2.3"}}, "version alias"},
		{"version", project.Catalog{Versions: project.Versions{"bad": "latest"}}, "not a v-prefixed"},
		{"dependency alias", project.Catalog{Libraries: project.CatalogDependencies{"Bad": project.CatalogLibrary("example.com/library", "v1.2.3")}}, "alias"},
		{"dependency module", project.Catalog{Libraries: project.CatalogDependencies{"bad": project.CatalogLibrary("../bad", "v1.2.3")}}, "module"},
		{"dependency version", project.Catalog{Starters: project.CatalogDependencies{"bad": project.CatalogStarter("example.com/starter", "missing")}}, "neither semantic"},
	}
	for _, test := range catalogTests {
		t.Run("catalog "+test.name, func(t *testing.T) {
			t.Parallel()
			assertErrorContains(t, project.ValidateCatalog(test.catalog), test.want)
		})
	}

	localTests := []struct {
		name  string
		local project.Local
		want  string
	}{
		{"module", project.Local{Replacements: project.Replacements{project.Replace("../bad", "../bad")}}, "module"},
		{"directory", project.Local{Replacements: project.Replacements{project.Replace("example.com/module", " bad ")}}, "directory"},
		{"duplicate", project.Local{Replacements: project.Replacements{project.Replace("example.com/module", "../one"), project.Replace("example.com/module", "../two")}}, "duplicated"},
		{"tool name", project.Local{ToolPaths: project.ToolPaths{"Bad": "tool"}}, "tool name"},
		{"tool path", project.Local{ToolPaths: project.ToolPaths{"tool": " bad\n"}}, "tool"},
		{"provider", project.Local{WorkspaceProvider: "native"}, "unsupported"},
	}
	for _, test := range localTests {
		t.Run("local "+test.name, func(t *testing.T) {
			t.Parallel()
			assertErrorContains(t, project.ValidateLocal(test.local), test.want)
		})
	}
}

func validSettings() project.Settings {
	return project.Settings{
		Name:   "commerce",
		Module: "example.com/commerce",
		Toolchain: project.Toolchain{
			Go:    "1.26.6",
			Spice: "v0.5.0",
		},
	}
}

func validBuild() project.Build {
	return project.Build{
		Kind:    project.Application,
		Plugins: project.Plugins{project.ApplicationPlugin()},
	}
}

func assertDependency(
	t *testing.T,
	dependency project.Dependency,
	kind project.DependencyKind,
	scope project.DependencyScope,
	name, module, version string,
) {
	t.Helper()
	if dependency.Kind != kind || dependency.Scope != scope || dependency.Name != name || dependency.Module != module || dependency.Version != version {
		t.Fatalf("dependency = %#v", dependency)
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want substring %q", err, want)
	}
}
