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

func TestCoreActionResolutionHonorsLabelOnlyScope(t *testing.T) {
	group, ok := coreActionGroupFor(true, ScrollAxisVertical, false)
	if !ok {
		t.Fatal("core action group is missing")
	}
	keyMap, err := NewKeyMap().Relabel(FocusNextActionID, "次へ移動")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveCoreActionGroup(
		"owner",
		group,
		[]KeyScope{NewKeyScope("localized", keyMap)},
	)
	if err != nil {
		t.Fatal(err)
	}
	actions := resolved.Actions()
	if actions[0].Label() != "次へ移動" {
		t.Fatalf("label = %q", actions[0].Label())
	}
	if len(actions[0].Bindings()) != len(group.descriptors[0].defaultBindings) {
		t.Fatalf("bindings = %#v", actions[0].Bindings())
	}
}
