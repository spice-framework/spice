package project_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spice-framework/spice/project"
	"github.com/spice-framework/spice/project/schema"
)

func TestModuleMetadataNormalizesAndRoundTrips(t *testing.T) {
	t.Parallel()

	input := validModuleMetadata()
	input.Capabilities = []string{"postgres", "database"}
	input.PublicPackages = []string{
		"example.com/starter-postgres/sql",
		"example.com/starter-postgres",
	}
	input.Documentation = []string{"https://example.com/postgres", "docs/postgres.md"}
	normalized, err := project.NewModuleMetadata(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(normalized.Capabilities, ",") != "database,postgres" {
		t.Fatalf("capabilities = %v", normalized.Capabilities)
	}
	input.Capabilities[0] = "mutated"
	if normalized.Capabilities[1] != "postgres" {
		t.Fatal("NewModuleMetadata retained caller-owned capability storage")
	}
	first, err := normalized.JSON()
	if err != nil {
		t.Fatal(err)
	}
	second, err := inputWithReorderedMetadata().JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("metadata JSON differs by input order:\n%s\n%s", first, second)
	}
	if first[len(first)-1] != '\n' {
		t.Fatal("metadata JSON has no final newline")
	}
	parsed, err := project.ParseModuleMetadata(first)
	if err != nil {
		t.Fatal(err)
	}
	parsedJSON, err := parsed.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, parsedJSON) {
		t.Fatalf("round-trip JSON differs:\n%s\n%s", first, parsedJSON)
	}
}

func TestParseModuleMetadataRejectsInvalidJSONBoundaries(t *testing.T) {
	t.Parallel()

	assertErrorContains(t, parseModuleMetadataError([]byte(`{"schema":1,"unknown":true}`)), "unknown field")
	valid, err := validModuleMetadata().JSON()
	if err != nil {
		t.Fatal(err)
	}
	assertErrorContains(t, parseModuleMetadataError(append(valid, []byte(`{}`)...)), "trailing JSON value")
	assertErrorContains(t, parseModuleMetadataError([]byte(`{"schema":`)), "decode Spice module metadata")
}

func TestModuleMetadataRejectsInvalidContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*project.ModuleMetadata)
		want   string
	}{
		{"schema", func(value *project.ModuleMetadata) { value.Schema = 2 }, "schema must"},
		{"kind", func(value *project.ModuleMetadata) { value.Kind = "unknown" }, "kind"},
		{"name", func(value *project.ModuleMetadata) { value.Name = "Postgres" }, "name"},
		{"module", func(value *project.ModuleMetadata) { value.Module = "../bad" }, "module path"},
		{"minimum", func(value *project.ModuleMetadata) { value.SpiceCompatibility.Minimum = "latest" }, "minimum"},
		{"current", func(value *project.ModuleMetadata) { value.SpiceCompatibility.Current = "latest" }, "current"},
		{"capability", func(value *project.ModuleMetadata) { value.Capabilities = []string{"Bad"} }, "capabilities"},
		{"capability duplicate", func(value *project.ModuleMetadata) { value.Capabilities = []string{"database", "database"} }, "duplicate"},
		{"starter", func(value *project.ModuleMetadata) { value.Starters = []string{"Bad"} }, "starters"},
		{"annotation ownership", func(value *project.ModuleMetadata) {
			value.AnnotationPackages = []string{"example.com/other/annotation"}
		}, "annotation packages"},
		{"configuration prefix", func(value *project.ModuleMetadata) { value.ConfigurationPrefixes = []string{"Spice.database"} }, "configuration prefixes"},
		{"compiler tool", func(value *project.ModuleMetadata) { value.CompilerTools = []string{"../tool"} }, "compiler tools"},
		{"public ownership", func(value *project.ModuleMetadata) { value.PublicPackages = []string{"example.com/other"} }, "public packages"},
		{"documentation scheme", func(value *project.ModuleMetadata) { value.Documentation = []string{"http://example.com/docs"} }, "documentation"},
		{"documentation path", func(value *project.ModuleMetadata) { value.Documentation = []string{"../docs"} }, "documentation"},
		{"generated requirement", func(value *project.ModuleMetadata) { value.GeneratedCodeRequirements = []string{"Bad"} }, "generated-code requirements"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			metadata := validModuleMetadata()
			test.mutate(&metadata)
			_, err := project.NewModuleMetadata(metadata)
			assertErrorContains(t, err, test.want)
			if _, jsonErr := metadata.JSON(); jsonErr == nil {
				t.Fatal("JSON accepted invalid metadata")
			}
		})
	}
}

func TestModuleMetadataAcceptsEveryPublishedKind(t *testing.T) {
	t.Parallel()

	for _, kind := range []project.ModuleKind{
		project.ModuleApplication,
		project.ModuleLibrary,
		project.ModulePlugin,
		project.ModuleStarter,
		project.ModuleTool,
	} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			metadata := validModuleMetadata()
			metadata.Kind = kind
			if _, err := project.NewModuleMetadata(metadata); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func validModuleMetadata() project.ModuleMetadata {
	return project.ModuleMetadata{
		Schema: schema.ModuleMetadata,
		Kind:   project.ModuleStarter,
		Name:   "postgres",
		Module: "example.com/starter-postgres",
		SpiceCompatibility: project.SpiceCompatibility{
			Minimum: "v0.5.0",
			Current: "v0.6.0",
		},
		Capabilities:          []string{"database", "postgres"},
		Starters:              []string{"postgres"},
		AnnotationPackages:    []string{"example.com/starter-postgres/annotation"},
		ConfigurationPrefixes: []string{"spice.datasource.postgres"},
		CompilerTools:         []string{"example.com/starter-postgres/cmd/compiler"},
		PublicPackages: []string{
			"example.com/starter-postgres",
			"example.com/starter-postgres/sql",
		},
		Documentation:             []string{"docs/postgres.md", "https://example.com/postgres"},
		GeneratedCodeRequirements: []string{"database", "migration"},
	}
}

func inputWithReorderedMetadata() project.ModuleMetadata {
	metadata := validModuleMetadata()
	metadata.Capabilities = []string{"postgres", "database"}
	metadata.PublicPackages = []string{
		"example.com/starter-postgres/sql",
		"example.com/starter-postgres",
	}
	metadata.Documentation = []string{"https://example.com/postgres", "docs/postgres.md"}
	return metadata
}

func parseModuleMetadataError(content []byte) error {
	_, err := project.ParseModuleMetadata(content)
	return err
}
