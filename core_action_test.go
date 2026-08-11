package tui

import "testing"

func TestCoreActionDescriptorsAreStaticOrderedAndAxisAware(t *testing.T) {
	group, ok := coreActionGroupFor(true, ScrollAxisHorizontal, true)
	if !ok {
		t.Fatal("core action group is missing")
	}
	descriptors := group.descriptors
	expected := []ActionID{
		FocusNextActionID,
		FocusPreviousActionID,
		ScrollPageUpActionID,
		ScrollPageDownActionID,
		ScrollStartActionID,
		ScrollEndActionID,
	}
	for index, id := range expected {
		if descriptors[index].id != id {
			t.Fatalf("descriptor %d ID = %s, want %s", index, descriptors[index].id, id)
		}
	}
	if descriptors[0].defaultBindings[0].repeatPolicy != RepeatAllow {
		t.Fatal("focus repeat policy is not RepeatAllow")
	}
	if descriptors[2].availability != ActionDisabledPassThrough ||
		descriptors[3].availability != ActionDisabledPassThrough {
		t.Fatal("horizontal page actions are not disabled-pass-through")
	}
	if descriptors[4].availability != ActionEnabled {
		t.Fatal("horizontal boundary action is not enabled")
	}

	allocations := testing.AllocsPerRun(1_000, func() {
		_, _ = coreActionGroupFor(true, ScrollAxisHorizontal, true)
	})
	if allocations != 0 {
		t.Fatalf("coreActionGroupFor allocations = %f, want 0", allocations)
	}
	resolvedAllocations := testing.AllocsPerRun(1_000, func() {
		_, _ = resolveCoreActionGroup("owner", group, nil)
	})
	if resolvedAllocations != 0 {
		t.Fatalf("resolveCoreActionGroup allocations = %f, want 0", resolvedAllocations)
	}
}
