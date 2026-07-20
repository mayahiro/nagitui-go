package widget

import "testing"

func TestTreeStateExpansionFollowsIdentityAfterReordering(t *testing.T) {
	items := []TreeItem{
		NewTreeBranch("a", "A", 0, true),
		NewTreeBranch("b", "B", 0, false),
	}
	state := NewTreeStateFromItems(items, 0)
	applied := state.Apply([]TreeItem{items[1], items[0]})
	if applied[0].Expanded || !applied[1].Expanded {
		t.Fatalf("Apply() = %#v", applied)
	}
}

func TestTreeStateApplyReturnsIndependentItems(t *testing.T) {
	items := []TreeItem{NewTreeBranch("a", "A", 0, false)}
	state := NewTreeState(0)
	state.SetExpanded("a", true)
	applied := state.Apply(items)
	if !applied[0].Expanded {
		t.Fatal("applied branch remained collapsed")
	}
	if items[0].Expanded {
		t.Fatal("Apply mutated source items")
	}
}
