package project_test

import (
	"fmt"
	"strings"

	"github.com/spice-framework/spice/project"
	"github.com/spice-framework/spice/project/schema"
)

func ExampleSettings() {
	settings := project.Settings{
		Name:   "commerce",
		Module: "github.com/acme/commerce",
		Toolchain: project.Toolchain{
			Go:    "1.26.6",
			Spice: "v0.5.0",
		},
		DependencyPolicy: project.DependencyPolicy{
			Verification: project.Strict,
		},
	}
	build := project.Build{
		Kind: project.Application,
		Dependencies: project.Dependencies{
			project.Starter("web"),
			project.Library("github.com/google/uuid", "v1.6.0"),
		},
		Plugins: project.Plugins{project.ApplicationPlugin()},
	}

	fmt.Println(project.ValidateSettings(settings))
	fmt.Println(project.ValidateBuild(build))
	// Output:
	// <nil>
	// <nil>
}

func ExampleModuleMetadata() {
	metadata, err := project.NewModuleMetadata(project.ModuleMetadata{
		Schema: schema.ModuleMetadata,
		Kind:   project.ModuleStarter,
		Name:   "postgres",
		Module: "github.com/spice-framework/starter-postgres",
		SpiceCompatibility: project.SpiceCompatibility{
			Minimum: "v0.5.0",
		},
		Capabilities: []string{"postgres", "database"},
		PublicPackages: []string{
			"github.com/spice-framework/starter-postgres",
		},
	})
	if err != nil {
		panic(err)
	}
	content, err := metadata.JSON()
	if err != nil {
		panic(err)
	}
	fmt.Println(strings.Contains(string(content), `"capabilities": [`))
	fmt.Println(metadata.Capabilities)
	// Output:
	// true
	// [database postgres]
}

func ExampleProjectModel_Agent() {
	model, err := project.NewProjectModel(project.ProjectModel{
		Schema: schema.ProjectModel,
		Project: project.ProjectIdentity{
			Name:   "commerce",
			Module: "github.com/acme/commerce",
			Kind:   project.Application,
		},
		SourceSets: []project.SourceSet{
			{ID: project.SourceSetMain, GoRoot: "src/main/go"},
		},
		Packages: []project.PackageRecord{
			{
				GoPackagePath: "github.com/acme/commerce/internal/users",
				GoPackageName: "users",
				Feature:       "users",
			},
		},
		Files: []project.FileRecord{
			{
				ID:            "users.UserService",
				CanonicalPath: "internal/users/user_service.go",
				ViewPath:      "src/main/go/users/application/UserService.go",
				GoPackagePath: "github.com/acme/commerce/internal/users",
				GoPackageName: "users",
				SourceSet:     project.SourceSetMain,
				Role:          project.RoleApplication,
				PrimarySymbol: "UserService",
				ContentHash:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			},
		},
	})
	if err != nil {
		panic(err)
	}
	agent, err := model.Agent()
	if err != nil {
		panic(err)
	}
	content, err := agent.JSON()
	if err != nil {
		panic(err)
	}
	fmt.Println(agent.Files[0].ViewPath)
	fmt.Println(strings.Contains(string(content), "canonicalPath"))
	// Output:
	// src/main/go/users/application/UserService.go
	// false
}
