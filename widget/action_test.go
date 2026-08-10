package widget

import (
	"testing"

	"github.com/mayahiro/nagitui-go"
)

var actionDescriptorSink tui.ActionDescriptor
var selectActionDescriptorSink [5]tui.ActionDescriptor
var tabsItemActionDescriptorSink tui.ActionDescriptor
var tabsNavigationActionDescriptorSink [4]tui.ActionDescriptor
var verticalCollectionActionDescriptorSink [5]tui.ActionDescriptor
var treeActionDescriptorSink [7]tui.ActionDescriptor
var listHasVisibleItemsSink bool

func TestActivateActionDescriptorReuseDoesNotAllocate(t *testing.T) {
	actionDescriptorSink = ActivateActionDescriptor()
	allocations := testing.AllocsPerRun(1_000, func() {
		actionDescriptorSink = ActivateActionDescriptor()
	})
	if allocations != 0 {
		t.Fatalf("descriptor allocations = %f, want 0", allocations)
	}
}

func TestSelectActionDescriptorDefaultsDoNotAllocate(t *testing.T) {
	selectActionDescriptorSink = selectActionDescriptors(true)
	allocations := testing.AllocsPerRun(1_000, func() {
		selectActionDescriptorSink = selectActionDescriptors(true)
	})
	if allocations != 0 {
		t.Fatalf("Select descriptor allocations = %f, want 0", allocations)
	}
}

func TestTabsActionDescriptorDefaultsDoNotAllocate(t *testing.T) {
	tabsItemActionDescriptorSink = tabsItemActionDescriptor(true)
	tabsNavigationActionDescriptorSink = tabsNavigationActionDescriptors(true)
	allocations := testing.AllocsPerRun(1_000, func() {
		tabsItemActionDescriptorSink = tabsItemActionDescriptor(true)
		tabsNavigationActionDescriptorSink = tabsNavigationActionDescriptors(true)
	})
	if allocations != 0 {
		t.Fatalf("Tabs descriptor allocations = %f, want 0", allocations)
	}
}

func TestVerticalCollectionActionDescriptorDefaultsDoNotAllocate(t *testing.T) {
	verticalCollectionActionDescriptorSink = verticalCollectionActionDescriptors(true)
	allocations := testing.AllocsPerRun(1_000, func() {
		verticalCollectionActionDescriptorSink = verticalCollectionActionDescriptors(true)
	})
	if allocations != 0 {
		t.Fatalf("vertical collection descriptor allocations = %f, want 0", allocations)
	}
}

func TestTreeActionDescriptorDefaultsDoNotAllocate(t *testing.T) {
	treeActionDescriptorSink = treeActionDescriptors(true)
	allocations := testing.AllocsPerRun(1_000, func() {
		treeActionDescriptorSink = treeActionDescriptors(true)
	})
	if allocations != 0 {
		t.Fatalf("Tree descriptor allocations = %f, want 0", allocations)
	}
}

func TestListDescriptorAvailabilityScanDoesNotAllocate(t *testing.T) {
	items := []ListItem{
		NewListItem("item-0", "Alpha"),
		NewListItem("item-1", "Beta"),
		NewListItem("item-2", "Alpine"),
	}
	listHasVisibleItemsSink = listHasVisibleItems(items, "p", 1, 1, true)
	allocations := testing.AllocsPerRun(1_000, func() {
		listHasVisibleItemsSink = listHasVisibleItems(items, "p", 1, 1, true)
	})
	if allocations != 0 {
		t.Fatalf("List availability scan allocations = %f, want 0", allocations)
	}
}

func TestListDescriptorAvailabilityUsesNormalizedWindow(t *testing.T) {
	list := NewList(
		tui.NewNodeID("list"),
		[]ListItem{NewListItem("item-0", "Alpha")},
		0,
		func(index int) int { return index },
	)
	tests := []struct {
		name         string
		offset       int
		limit        int
		availability tui.ActionAvailability
	}{
		{name: "negative offset", offset: -1, limit: 1, availability: tui.ActionEnabled},
		{name: "negative limit", offset: 0, limit: -1, availability: tui.ActionDisabledPassThrough},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, descriptor := range list.Window(test.offset, test.limit).ActionDescriptors() {
				if actual := descriptor.Availability(); actual != test.availability {
					t.Fatalf("availability = %v, want %v", actual, test.availability)
				}
			}
		})
	}
}
