package schema_test

import (
	"testing"

	"github.com/spice-framework/spice/project/schema"
)

func TestSchemaIdentities(t *testing.T) {
	t.Parallel()

	if schema.ProjectModel != "spice.project-model/v1alpha1" {
		t.Fatalf("ProjectModel = %q", schema.ProjectModel)
	}
	if schema.AgentProjectModel != "spice.project-model.agent/v1alpha1" {
		t.Fatalf("AgentProjectModel = %q", schema.AgentProjectModel)
	}
	if schema.WorkspaceProtocol != "spice.workspace/v1alpha1" {
		t.Fatalf("WorkspaceProtocol = %q", schema.WorkspaceProtocol)
	}
	if schema.ModuleMetadata != 1 {
		t.Fatalf("ModuleMetadata = %d", schema.ModuleMetadata)
	}
}
