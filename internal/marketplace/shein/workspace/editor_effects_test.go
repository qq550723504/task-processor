package workspace

import "testing"

func TestBuildCategoryEffects(t *testing.T) {
	effects := BuildCategoryEffects()
	if len(effects) != 1 || effects[0].Key != "category_resolution" {
		t.Fatalf("effects = %#v", effects)
	}
}
