package widget

import (
	"testing"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

func TestResolvedHelpFiltersHiddenActionsAndDisablesUnsupportedBindings(t *testing.T) {
	visible := tui.NewActionDescriptor(
		tui.NewActionID("app.visible"),
		"Visible",
		[]tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('u', vt.Modifiers{})).
				WithSupport(tui.BindingUnsupported),
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('v', vt.Modifiers{})),
		},
	)
	hidden := tui.NewActionDescriptor(
		tui.NewActionID("app.hidden"),
		"Hidden",
		[]tui.KeyBinding{
			tui.NewKeyBinding(tui.NewCharacterKeyStroke('h', vt.Modifiers{})),
		},
	).WithHelpVisible(false)
	resolved, err := tui.ResolveActions(
		tui.NewNodeID("owner"),
		[]tui.ActionDescriptor{visible, hidden},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	help := NewHelpFromResolvedActions[struct{}](resolved.Actions())

	if len(help.bindings) != 2 {
		t.Fatalf("binding count = %d, want 2", len(help.bindings))
	}
	for index, expected := range []HelpBinding{
		{Key: "u", Description: "Visible", Enabled: false},
		{Key: "v", Description: "Visible", Enabled: true},
	} {
		if actual := help.bindings[index]; actual != expected {
			t.Errorf("binding %d = %#v, want %#v", index, actual, expected)
		}
	}
}
