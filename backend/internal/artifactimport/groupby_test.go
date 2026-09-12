package artifactimport

import (
	"testing"

	"github.com/google/uuid"
)

func TestGroupByProjectPreservesOrder(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	plan := []PreparedUpload{
		{ProjectID: first, ProjectName: "first"},
		{ProjectID: second, ProjectName: "second"},
		{ProjectID: first, ProjectName: "first"},
		{ProjectID: second, ProjectName: "second"},
		{ProjectID: first, ProjectName: "first"},
	}
	groups := groupByProject(plan)
	if len(groups) != 2 {
		t.Fatalf("groupByProject produced %d groups, expected 2", len(groups))
	}
	if groups[0].ID != first || groups[1].ID != second {
		t.Fatalf("groupByProject dispatched in order %v %v", groups[0].ID, groups[1].ID)
	}
	if len(groups[0].Uploads) != 3 || len(groups[1].Uploads) != 2 {
		t.Fatalf("groupByProject split files %d and %d, expected 3 and 2", len(groups[0].Uploads), len(groups[1].Uploads))
	}
}
