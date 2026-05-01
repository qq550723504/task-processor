package listingkit

import (
	"testing"

	sheincategory "task-processor/internal/shein/api/category"
)

func TestSearchSheinCategoryCandidatesMatchesLeafAndPath(t *testing.T) {
	t.Parallel()

	tree := []sheincategory.CategoryTreeNode{
		{
			CategoryID:    10,
			CategoryName:  "Home & Living",
			ProductTypeID: 100,
			Children: []sheincategory.CategoryTreeNode{
				{
					CategoryID:    11,
					CategoryName:  "Sleep",
					ProductTypeID: 101,
					Children: []sheincategory.CategoryTreeNode{
						{
							CategoryID:    12,
							CategoryName:  "Sleep Masks",
							ProductTypeID: 102,
							LastCategory:  true,
						},
					},
				},
			},
		},
	}

	results := searchSheinCategoryCandidates(tree, "sleep mask")
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].CategoryID != 12 {
		t.Fatalf("category id = %d, want 12", results[0].CategoryID)
	}
	if got := results[0].CategoryPath; len(got) != 3 || got[2] != "Sleep Masks" {
		t.Fatalf("category path = %#v", got)
	}
	if results[0].TopCategoryID != 10 {
		t.Fatalf("top category id = %d, want 10", results[0].TopCategoryID)
	}
}

func TestSearchSheinCategoryCandidatesPrefersBetterLeafMatch(t *testing.T) {
	t.Parallel()

	tree := []sheincategory.CategoryTreeNode{
		{
			CategoryID:   1,
			CategoryName: "Sports",
			Children: []sheincategory.CategoryTreeNode{
				{CategoryID: 2, CategoryName: "Mask Accessories", LastCategory: true},
				{CategoryID: 3, CategoryName: "Sports Mask", LastCategory: true},
			},
		},
	}

	results := searchSheinCategoryCandidates(tree, "sports")
	if len(results) < 2 {
		t.Fatalf("result count = %d, want at least 2", len(results))
	}
	if results[0].CategoryID != 3 {
		t.Fatalf("first category id = %d, want 3", results[0].CategoryID)
	}
}
