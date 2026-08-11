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
var composerLeadingActionDescriptorSink [3]tui.ActionDescriptor
var listHasVisibleItemsSink bool
var commandPaletteActionDescriptorSink [5]tui.ActionDescriptor
var commandPaletteCommandActionDescriptorSink tui.ActionDescriptor
var commandPaletteHasVisibleCommandSink bool
var paginatorActionDescriptorSink [4]tui.ActionDescriptor
var filePickerActionDescriptorSink [filePickerActionCount]tui.ActionDescriptor

func nodeDeclaredActionGroups(groups []tui.ResolvedActions) []tui.ResolvedActions {
	declared := make([]tui.ResolvedActions, 0, len(groups))
	for _, group := range groups {
		actions := group.Actions()
		if len(actions) != 0 &&
			(actions[0].ID() == tui.FocusNextActionID || actions[0].ID() == tui.ScrollPageUpActionID) {
			continue
		}
		declared = append(declared, group)
	}
	return declared
}

func TestActivateActionDescriptorReuseDoesNotAllocate(t *testing.T) {
	actionDescriptorSink = ActivateActionDescriptor()
	allocations := testing.AllocsPerRun(1_000, func() {
		actionDescriptorSink = ActivateActionDescriptor()
	})
	if allocations != 0 {
		t.Fatalf("descriptor allocations = %f, want 0", allocations)
	}
}

func TestDismissActionDescriptorReuseDoesNotAllocate(t *testing.T) {
	actionDescriptorSink = DismissActionDescriptor()
	allocations := testing.AllocsPerRun(1_000, func() {
		actionDescriptorSink = DismissActionDescriptor()
	})
	if allocations != 0 {
		t.Fatalf("descriptor allocations = %f, want 0", allocations)
	}
}

func TestConfirmActionDescriptorReuseDoesNotAllocate(t *testing.T) {
	actionDescriptorSink = ConfirmActionDescriptor()
	allocations := testing.AllocsPerRun(1_000, func() {
		actionDescriptorSink = ConfirmActionDescriptor()
	})
	if allocations != 0 {
		t.Fatalf("descriptor allocations = %f, want 0", allocations)
	}
}

func TestPaginatorActionDescriptorDefaultsDoNotAllocate(t *testing.T) {
	paginatorActionDescriptorSink = paginatorActionDescriptors(true)
	allocations := testing.AllocsPerRun(1_000, func() {
		paginatorActionDescriptorSink = paginatorActionDescriptors(true)
	})
	if allocations != 0 {
		t.Fatalf("Paginator descriptor allocations = %f, want 0", allocations)
	}
}

func TestFilePickerActionDescriptorDefaultsDoNotAllocate(t *testing.T) {
	filePickerActionDescriptorSink = filePickerActionDescriptors(true, true, true)
	allocations := testing.AllocsPerRun(1_000, func() {
		filePickerActionDescriptorSink = filePickerActionDescriptors(true, true, true)
	})
	if allocations != 0 {
		t.Fatalf("FilePicker descriptor allocations = %f, want 0", allocations)
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
	textAreaActionDescriptorSink = textAreaActionDescriptors(
		true, true, true, TextAreaBoundaryConsume, TextAreaState{}, 0, false,
	)
	allocations := testing.AllocsPerRun(1_000, func() {
		textAreaActionDescriptorSink = textAreaActionDescriptors(
			true, true, true, TextAreaBoundaryConsume, TextAreaState{}, 0, false,
		)
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

func TestComposerLeadingActionDescriptorDefaultsDoNotAllocate(t *testing.T) {
	composerLeadingActionDescriptorSink = composerLeadingActionDescriptors(true, true, true, true)
	allocations := testing.AllocsPerRun(1_000, func() {
		composerLeadingActionDescriptorSink = composerLeadingActionDescriptors(true, true, true, true)
	})
	if allocations != 0 {
		t.Fatalf("Composer leading descriptor allocations = %f, want 0", allocations)
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
