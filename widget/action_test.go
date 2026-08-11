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
var textAreaActionDescriptorSink [textAreaActionCount]tui.ActionDescriptor
var listHasVisibleItemsSink bool
var commandPaletteActionDescriptorSink [5]tui.ActionDescriptor
var commandPaletteCommandActionDescriptorSink tui.ActionDescriptor
var commandPaletteHasVisibleCommandSink bool

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

func TestTextAreaActionDescriptorDefaultsDoNotAllocate(t *testing.T) {
	textAreaActionDescriptorSink = textAreaActionDescriptors(true, true, true)
	allocations := testing.AllocsPerRun(1_000, func() {
		textAreaActionDescriptorSink = textAreaActionDescriptors(true, true, true)
	})
	if allocations != 0 {
		t.Fatalf("TextArea descriptor allocations = %f, want 0", allocations)
	}
	for _, descriptor := range textAreaActionDescriptorSink {
		for _, binding := range descriptor.DefaultBindings() {
			if binding.RepeatPolicy() != tui.RepeatAllow {
				t.Fatalf("TextArea action %s does not allow repeat", descriptor.ID())
			}
		}
	}
}

func TestCommandPaletteActionDescriptorDefaultsDoNotAllocate(t *testing.T) {
	commandPaletteCommandActionDescriptorSink = commandPaletteCommandActionDescriptor(true)
	commandPaletteActionDescriptorSink = commandPaletteActionDescriptors(true)
	allocations := testing.AllocsPerRun(1_000, func() {
		commandPaletteCommandActionDescriptorSink = commandPaletteCommandActionDescriptor(true)
		commandPaletteActionDescriptorSink = commandPaletteActionDescriptors(true)
	})
	if allocations != 0 {
		t.Fatalf("CommandPalette descriptor allocations = %f, want 0", allocations)
	}
}

func TestCommandPaletteDescriptorAvailabilityScanDoesNotAllocate(t *testing.T) {
	commands := []Command{
		NewCommand("command-0", "Alpha").WithKeywords("first"),
		NewCommand("command-1", "Beta").WithKeywords("second"),
		NewCommand("command-2", "Alpine").WithKeywords("peak"),
	}
	commandPaletteHasVisibleCommandSink = hasVisibleCommand(commands, "P")
	allocations := testing.AllocsPerRun(1_000, func() {
		commandPaletteHasVisibleCommandSink = hasVisibleCommand(commands, "P")
	})
	if allocations != 0 {
		t.Fatalf("CommandPalette availability scan allocations = %f, want 0", allocations)
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
