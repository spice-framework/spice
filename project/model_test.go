package project_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spice-framework/spice/project"
	"github.com/spice-framework/spice/project/schema"
)

func TestProjectModelNormalizesRoundTripsAndProjectsForAgents(t *testing.T) {
	t.Parallel()

	model := validProjectModel()
	model.Files[0], model.Files[1] = model.Files[1], model.Files[0]
	model.Dependencies[0].Capabilities = []string{"web.server", "http"}
	normalized, err := project.NewProjectModel(model)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Files[0].ViewPath != "build/generated/spice/commerce/spice_assembly_gen.go" {
		t.Fatalf("first View file = %q", normalized.Files[0].ViewPath)
	}
	if strings.Join(normalized.Dependencies[0].Capabilities, ",") != "http,web.server" {
		t.Fatalf("capabilities = %v", normalized.Dependencies[0].Capabilities)
	}
	model.Dependencies[0].Capabilities[0] = "mutated"
	if normalized.Dependencies[0].Capabilities[1] != "web.server" {
		t.Fatal("NewProjectModel retained caller-owned dependency capability storage")
	}
	first, err := normalized.JSON()
	if err != nil {
		t.Fatal(err)
	}
	second, err := validProjectModel().JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("Project Model JSON differs by input order:\n%s\n%s", first, second)
	}
	parsed, err := project.ParseProjectModel(first)
	if err != nil {
		t.Fatal(err)
	}
	parsedJSON, err := parsed.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, parsedJSON) {
		t.Fatalf("Project Model JSON round trip differs:\n%s\n%s", first, parsedJSON)
	}

	agent, err := normalized.Agent()
	if err != nil {
		t.Fatal(err)
	}
	if agent.Schema != schema.AgentProjectModel {
		t.Fatalf("agent schema = %q", agent.Schema)
	}
	agentJSON, err := agent.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(agentJSON, []byte("canonicalPath")) || bytes.Contains(agentJSON, []byte("internal/users/user_service.go")) {
		t.Fatalf("agent JSON leaked canonical path:\n%s", agentJSON)
	}
	if !bytes.Contains(agentJSON, []byte("src/main/go/users/application/UserService.go")) {
		t.Fatalf("agent JSON omits View path:\n%s", agentJSON)
	}
	parsedAgent, err := project.ParseAgentProjectModel(agentJSON)
	if err != nil {
		t.Fatal(err)
	}
	parsedAgentJSON, err := parsedAgent.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(agentJSON, parsedAgentJSON) {
		t.Fatalf("agent Project Model JSON round trip differs:\n%s\n%s", agentJSON, parsedAgentJSON)
	}
}

func TestProjectModelPathMappingIsBijective(t *testing.T) {
	t.Parallel()

	model, err := project.NewProjectModel(validProjectModel())
	if err != nil {
		t.Fatal(err)
	}
	canonicalToView := make(map[string]string, len(model.Files))
	viewToCanonical := make(map[string]string, len(model.Files))
	for _, file := range model.Files {
		canonicalToView[file.CanonicalPath] = file.ViewPath
		viewToCanonical[file.ViewPath] = file.CanonicalPath
	}
	for canonical, view := range canonicalToView {
		if got := viewToCanonical[view]; got != canonical {
			t.Fatalf("%q -> %q -> %q", canonical, view, got)
		}
	}
}

func TestParseProjectModelRejectsJSONBoundaries(t *testing.T) {
	t.Parallel()

	assertErrorContains(t, parseProjectModelError([]byte(`{"schema":"spice.project-model/v1alpha1","unknown":true}`)), "unknown field")
	valid, err := validProjectModel().JSON()
	if err != nil {
		t.Fatal(err)
	}
	assertErrorContains(t, parseProjectModelError(append(valid, []byte(`{}`)...)), "trailing JSON value")
	assertErrorContains(t, parseProjectModelError([]byte(`{"schema":`)), "decode Spice Project Model")
}

func TestProjectModelRejectsInvalidContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*project.ProjectModel)
		want   string
	}{
		{"schema", func(value *project.ProjectModel) { value.Schema = "spice.project-model/v2" }, "schema must"},
		{"project name", func(value *project.ProjectModel) { value.Project.Name = "Commerce" }, "project name"},
		{"project module", func(value *project.ProjectModel) { value.Project.Module = "../bad" }, "module"},
		{"project kind", func(value *project.ProjectModel) { value.Project.Kind = "unknown" }, "kind"},
		{"source set id", func(value *project.ProjectModel) { value.SourceSets[0].ID = "custom" }, "source set"},
		{"source roots empty", func(value *project.ProjectModel) {
			value.SourceSets[0].GoRoot = ""
			value.SourceSets[0].ResourceRoot = ""
		}, "has no roots"},
		{"source set duplicate", func(value *project.ProjectModel) { value.SourceSets = append(value.SourceSets, value.SourceSets[0]) }, "duplicated"},
		{"source root path", func(value *project.ProjectModel) { value.SourceSets[0].GoRoot = "../src" }, "source root"},
		{"source root collision", func(value *project.ProjectModel) { value.SourceSets[0].GoRoot = "SRC/MAIN/GO" }, "case folding"},
		{"package ownership", func(value *project.ProjectModel) { value.Packages[0].GoPackagePath = "example.com/other" }, "does not belong"},
		{"package name", func(value *project.ProjectModel) { value.Packages[0].GoPackageName = "Users" }, "package name"},
		{"package feature", func(value *project.ProjectModel) { value.Packages[0].Feature = "Bad" }, "feature"},
		{"application module", func(value *project.ProjectModel) { value.Packages[0].Module = "../bad" }, "application module"},
		{"package duplicate", func(value *project.ProjectModel) { value.Packages = append(value.Packages, value.Packages[0]) }, "Go package"},
		{"View path", func(value *project.ProjectModel) { value.Views[0].Path = "../bad" }, "invalid identity or path"},
		{"View role", func(value *project.ProjectModel) { value.Views[0].Role = "unknown" }, "role"},
		{"View source set", func(value *project.ProjectModel) { value.Views[0].SourceSet = "unknown" }, "unknown source set"},
		{"View package", func(value *project.ProjectModel) {
			value.Views[0].GoPackagePath = "example.com/commerce/internal/missing"
		}, "unknown Go package"},
		{"View feature", func(value *project.ProjectModel) { value.Views[0].Feature = "Bad" }, "feature"},
		{"View ID duplicate", func(value *project.ProjectModel) {
			value.Views = append(value.Views, value.Views[0])
			value.Views[len(value.Views)-1].Path = "src/main/go/users/domain"
		}, "View ID"},
		{"View path collision", func(value *project.ProjectModel) {
			value.Views = append(value.Views, value.Views[0])
			value.Views[len(value.Views)-1].ID = "users/domain"
			value.Views[len(value.Views)-1].Path = strings.ToUpper(value.Views[0].Path)
		}, "case folding"},
		{"file ID", func(value *project.ProjectModel) { value.Files[0].ID = " " }, "trimmed non-empty ID"},
		{"file canonical path", func(value *project.ProjectModel) { value.Files[0].CanonicalPath = "../bad" }, "invalid canonical"},
		{"file reserved path", func(value *project.ProjectModel) { value.Files[0].ViewPath = "src/main/go/users/domain/CON.go" }, "invalid canonical"},
		{"file source set", func(value *project.ProjectModel) { value.Files[0].SourceSet = "unknown" }, "unknown source set"},
		{"file role", func(value *project.ProjectModel) { value.Files[0].Role = "unknown" }, "role"},
		{"file package", func(value *project.ProjectModel) { value.Files[0].GoPackagePath = "example.com/missing" }, "unknown Go package"},
		{"file package name", func(value *project.ProjectModel) { value.Files[0].GoPackageName = "wrong" }, "does not match"},
		{"file primary symbol", func(value *project.ProjectModel) { value.Files[0].PrimarySymbol = "bad-name" }, "primary symbol"},
		{"generated writable", func(value *project.ProjectModel) { value.Files[1].ReadOnly = false }, "generated file"},
		{"generated source set", func(value *project.ProjectModel) { value.Files[1].SourceSet = project.SourceSetMain }, "generated file"},
		{"file hash", func(value *project.ProjectModel) { value.Files[0].ContentHash = strings.Repeat("A", 64) }, "content hash"},
		{"file ID duplicate", func(value *project.ProjectModel) {
			value.Files = append(value.Files, value.Files[0])
			value.Files[len(value.Files)-1].CanonicalPath = "internal/users/other.go"
			value.Files[len(value.Files)-1].ViewPath = "src/main/go/users/domain/Other.go"
		}, "file ID"},
		{"canonical collision", func(value *project.ProjectModel) {
			value.Files = append(value.Files, value.Files[0])
			value.Files[len(value.Files)-1].ID = "users.Other"
			value.Files[len(value.Files)-1].CanonicalPath = strings.ToUpper(value.Files[0].CanonicalPath)
			value.Files[len(value.Files)-1].ViewPath = "src/main/go/users/domain/Other.go"
		}, "canonical path"},
		{"View file collision", func(value *project.ProjectModel) {
			value.Files = append(value.Files, value.Files[0])
			value.Files[len(value.Files)-1].ID = "users.Other"
			value.Files[len(value.Files)-1].CanonicalPath = "internal/users/other.go"
			value.Files[len(value.Files)-1].ViewPath = strings.ToUpper(value.Files[0].ViewPath)
		}, "View path"},
		{"dependency ID", func(value *project.ProjectModel) { value.Dependencies[0].ID = " " }, "trimmed non-empty ID"},
		{"dependency kind", func(value *project.ProjectModel) { value.Dependencies[0].Kind = "unknown" }, "kind"},
		{"dependency scope", func(value *project.ProjectModel) { value.Dependencies[0].Scope = "unknown" }, "scope"},
		{"dependency module", func(value *project.ProjectModel) { value.Dependencies[0].Module = "../bad" }, "invalid resolved"},
		{"starter name", func(value *project.ProjectModel) { value.Dependencies[0].Name = "Bad" }, "starter dependency"},
		{"library name", func(value *project.ProjectModel) { value.Dependencies[0].Kind = project.DependencyLibrary }, "non-starter"},
		{"dependency capability", func(value *project.ProjectModel) { value.Dependencies[0].Capabilities = []string{"Bad"} }, "capabilities"},
		{"dependency duplicate", func(value *project.ProjectModel) {
			value.Dependencies = append(value.Dependencies, value.Dependencies[0])
		}, "dependency ID"},
		{"target name", func(value *project.ProjectModel) { value.Targets[0].Name = "Bad" }, "invalid name"},
		{"target package", func(value *project.ProjectModel) { value.Targets[0].GoPackagePath = "example.com/missing" }, "unknown Go package"},
		{"target generated", func(value *project.ProjectModel) { value.Targets[0].GeneratedGoPackagePath = "../bad" }, "generated Go package"},
		{"target generated package", func(value *project.ProjectModel) {
			value.Targets[0].GeneratedGoPackagePath = "example.com/commerce/internal/spicegen/missing"
		}, "unknown generated Go package"},
		{"target duplicate", func(value *project.ProjectModel) { value.Targets = append(value.Targets, value.Targets[0]) }, "target"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			model := validProjectModel()
			test.mutate(&model)
			_, err := project.NewProjectModel(model)
			assertErrorContains(t, err, test.want)
		})
	}
}

func TestAgentProjectModelRejectsWrongSchema(t *testing.T) {
	t.Parallel()

	_, err := (project.AgentProjectModel{Schema: "wrong"}).JSON()
	assertErrorContains(t, err, "agent Project Model schema")
	model := validProjectModel()
	model.Schema = "wrong"
	_, err = model.Agent()
	assertErrorContains(t, err, "project agent model")
}

func TestAgentProjectModelRejectsInvalidContracts(t *testing.T) {
	t.Parallel()

	complete, err := validProjectModel().Agent()
	if err != nil {
		t.Fatal(err)
	}
	complete.Files[0].ViewPath = "C:/physical/source.go"
	_, err = project.NewAgentProjectModel(complete)
	assertErrorContains(t, err, "invalid View path")

	valid, err := validProjectModel().Agent()
	if err != nil {
		t.Fatal(err)
	}
	content, err := valid.JSON()
	if err != nil {
		t.Fatal(err)
	}
	assertErrorContains(t, parseAgentProjectModelError(append(content, []byte(`{}`)...)), "trailing JSON value")
	assertErrorContains(t, parseAgentProjectModelError([]byte(`{"schema":"spice.project-model.agent/v1alpha1","unknown":true}`)), "unknown field")
}

func TestProjectModelJSONDoesNotExposeAbsolutePaths(t *testing.T) {
	t.Parallel()

	content, err := validProjectModel().JSON()
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(content, []byte(`D:\\`)) || bytes.Contains(content, []byte(`/Users/`)) {
		t.Fatalf("Project Model contains an absolute path:\n%s", content)
	}
}

func validProjectModel() project.ProjectModel {
	const hash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	return project.ProjectModel{
		Schema: schema.ProjectModel,
		Project: project.ProjectIdentity{
			Name:   "commerce",
			Module: "example.com/commerce",
			Kind:   project.Application,
		},
		SourceSets: []project.SourceSet{
			{ID: project.SourceSetTest, GoRoot: "src/test/go", ResourceRoot: "src/test/resources"},
			{ID: project.SourceSetMain, GoRoot: "src/main/go", ResourceRoot: "src/main/resources"},
			{ID: project.SourceSetGenerated, GoRoot: "build/generated/spice"},
		},
		Packages: []project.PackageRecord{
			{GoPackagePath: "example.com/commerce/internal/users", GoPackageName: "users", Feature: "users", Module: "example.com/commerce/internal/users"},
			{GoPackagePath: "example.com/commerce/cmd/commerce", GoPackageName: "main", Module: "example.com/commerce/cmd/commerce"},
			{GoPackagePath: "example.com/commerce/internal/spicegen/commerce", GoPackageName: "commerce", Module: "example.com/commerce/cmd/commerce"},
		},
		Views: []project.ViewRecord{
			{ID: "users/application", Path: "src/main/go/users/application", Feature: "users", Role: project.RoleApplication, SourceSet: project.SourceSetMain, GoPackagePath: "example.com/commerce/internal/users"},
			{ID: "commerce/generated", Path: "build/generated/spice/commerce", Role: project.RoleGenerated, SourceSet: project.SourceSetGenerated, GoPackagePath: "example.com/commerce/internal/spicegen/commerce"},
		},
		Files: []project.FileRecord{
			{
				ID:            "users.UserService",
				CanonicalPath: "internal/users/user_service.go",
				ViewPath:      "src/main/go/users/application/UserService.go",
				GoPackagePath: "example.com/commerce/internal/users",
				GoPackageName: "users",
				SourceSet:     project.SourceSetMain,
				Role:          project.RoleApplication,
				PrimarySymbol: "UserService",
				ContentHash:   hash,
			},
			{
				ID:            "commerce.generated.assembly",
				CanonicalPath: "internal/spicegen/commerce/spice_assembly_gen.go",
				ViewPath:      "build/generated/spice/commerce/spice_assembly_gen.go",
				GoPackagePath: "example.com/commerce/internal/spicegen/commerce",
				GoPackageName: "commerce",
				SourceSet:     project.SourceSetGenerated,
				Role:          project.RoleGenerated,
				Generated:     true,
				ReadOnly:      true,
				ContentHash:   hash,
			},
		},
		Dependencies: []project.ResolvedDependency{
			{
				ID:           "starter:web",
				Kind:         project.DependencyStarter,
				Scope:        project.ScopeMain,
				Name:         "web",
				Module:       "example.com/starter-web",
				Version:      "v1.2.3",
				Direct:       true,
				Capabilities: []string{"http", "web.server"},
			},
		},
		Targets: []project.TargetRecord{
			{
				Name:                   "commerce",
				Kind:                   project.Application,
				GoPackagePath:          "example.com/commerce/cmd/commerce",
				GeneratedGoPackagePath: "example.com/commerce/internal/spicegen/commerce",
			},
		},
	}
}

func parseProjectModelError(content []byte) error {
	_, err := project.ParseProjectModel(content)
	return err
}

func parseAgentProjectModelError(content []byte) error {
	_, err := project.ParseAgentProjectModel(content)
	return err
}
