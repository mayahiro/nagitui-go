package tui

import (
	"errors"
	"testing"

	"github.com/mayahiro/nagi-go/vt"
)

func TestKeyBindingIgnoresAssociatedTextProtocolAndSupport(t *testing.T) {
	stroke := NewCharacterKeyStroke('a', vt.Modifiers{Control: true})
	binding := NewKeyBinding(stroke).WithSupport(BindingUnsupported)
	event := vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: vt.KeyCharacter, Character: 'a', Modifiers: stroke.Modifiers(),
		Action: vt.KeyPress, Text: "different", HasText: true,
		Protocol: vt.KeyProtocolUnknown,
	}}

	if !binding.Matches(event) {
		t.Fatal("binding did not match equivalent logical key")
	}
}

func TestDuplicateOverrideDoesNotMutateOriginalMap(t *testing.T) {
	original, err := NewKeyMap().Rebind(
		NewActionID("app.submit"),
		[]KeyBinding{NewKeyBinding(NewKeyStroke(vt.KeyEnter, vt.Modifiers{}))},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = original.Rebind(NewActionID("app.submit"), nil)
	var duplicate *DuplicateActionOverrideError
	if !errors.As(err, &duplicate) {
		t.Fatalf("error = %T %v", err, err)
	}
	bindings, ok := original.Bindings(NewActionID("app.submit"))
	if !ok || len(bindings) != 1 {
		t.Fatalf("original bindings = %v, %t", bindings, ok)
	}
}

func TestKeyMapOwnsBindingSlices(t *testing.T) {
	action := NewActionID("app.submit")
	enter := NewKeyBinding(NewKeyStroke(vt.KeyEnter, vt.Modifiers{}))
	up := NewKeyBinding(NewKeyStroke(vt.KeyUp, vt.Modifiers{}))
	input := []KeyBinding{enter}
	keyMap, err := NewKeyMap().Rebind(action, input)
	if err != nil {
		t.Fatal(err)
	}

	input[0] = up
	bindings, ok := keyMap.Bindings(action)
	if !ok || len(bindings) != 1 || bindings[0] != enter {
		t.Fatalf("stored bindings = %v, %t", bindings, ok)
	}
	bindings[0] = up
	bindings, ok = keyMap.Bindings(action)
	if !ok || len(bindings) != 1 || bindings[0] != enter {
		t.Fatalf("bindings after returned-slice mutation = %v, %t", bindings, ok)
	}
}

func TestKeyMapBindingAndLabelOverridesAreIndependent(t *testing.T) {
	action := NewActionID("app.submit")
	keyMap, err := NewKeyMap().Rebind(
		action,
		[]KeyBinding{NewKeyBinding(NewKeyStroke(vt.KeyEnter, vt.Modifiers{}))},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyMap, err = keyMap.Relabel(action, "Send")
	if err != nil {
		t.Fatal(err)
	}

	bindings, ok := keyMap.Bindings(action)
	if !ok || len(bindings) != 1 {
		t.Fatalf("bindings = %v, %t", bindings, ok)
	}
	if label, ok := keyMap.Label(action); !ok || label != "Send" {
		t.Fatalf("label = %q, %t", label, ok)
	}
	if keyMap.Empty() {
		t.Fatal("KeyMap with overrides reports empty")
	}
}

func TestDuplicateLabelOverrideDoesNotMutateOriginalMap(t *testing.T) {
	action := NewActionID("app.submit")
	original, err := NewKeyMap().Relabel(action, "Send")
	if err != nil {
		t.Fatal(err)
	}
	_, err = original.Relabel(action, "Submit")
	var duplicate *DuplicateActionLabelOverrideError
	if !errors.As(err, &duplicate) {
		t.Fatalf("error = %T %v", err, err)
	}
	if duplicate.Action != action {
		t.Fatalf("duplicate Action = %q", duplicate.Action)
	}
	if label, ok := original.Label(action); !ok || label != "Send" {
		t.Fatalf("original label = %q, %t", label, ok)
	}
}

func TestNearestLabelOverrideWinsWithoutChangingBindings(t *testing.T) {
	action := NewActionDescriptor(
		"app.submit",
		"Submit",
		[]KeyBinding{NewKeyBinding(NewKeyStroke(vt.KeyEnter, vt.Modifiers{}))},
	)
	outer, err := NewKeyMap().Relabel("app.submit", "Send")
	if err != nil {
		t.Fatal(err)
	}
	inner, err := NewKeyMap().Relabel("app.submit", "送信")
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveActions(
		"composer",
		[]ActionDescriptor{action},
		[]KeyScope{NewKeyScope("outer", outer), NewKeyScope("inner", inner)},
	)
	if err != nil {
		t.Fatal(err)
	}
	actions := resolved.Actions()
	if len(actions) != 1 || actions[0].Label() != "送信" || len(actions[0].Bindings()) != 1 {
		t.Fatalf("resolved actions = %#v", actions)
	}
	help := resolved.HelpActions()
	if len(help) != 1 || help[0].Label() != "送信" {
		t.Fatalf("HelpActions = %#v", help)
	}
}

func TestKeyBindingMatchDoesNotAllocate(t *testing.T) {
	stroke := NewCharacterKeyStroke('a', vt.Modifiers{Control: true})
	binding := NewKeyBinding(stroke)
	event := vt.Event{Kind: vt.EventKey, Key: vt.KeyEvent{
		Code: vt.KeyCharacter, Character: 'a', Modifiers: stroke.Modifiers(),
		Action: vt.KeyPress,
	}}
	matched := false
	allocations := testing.AllocsPerRun(1_000, func() {
		matched = binding.Matches(event)
	})
	if !matched {
		t.Fatal("binding did not match equivalent logical key")
	}
	if allocations != 0 {
		t.Fatalf("Matches allocations = %f, want 0", allocations)
	}
}

func TestKeyStrokeFromEventRejectsInvalidUTF8(t *testing.T) {
	_, ok := KeyStrokeFromEvent(vt.Event{Kind: vt.EventText, Text: string([]byte{0xff})})
	if ok {
		t.Fatal("invalid UTF-8 Text produced a KeyStroke")
	}
}

func TestKeyStrokeIdentityIgnoresLocksAndDistinguishesExtendedModifiers(t *testing.T) {
	withLocks := NewKeyStroke(vt.KeyEnter, vt.Modifiers{Super: true, CapsLock: true, NumLock: true})
	withoutLocks := NewKeyStroke(vt.KeyEnter, vt.Modifiers{Super: true})
	hyper := NewKeyStroke(vt.KeyEnter, vt.Modifiers{Hyper: true})
	if withLocks != withoutLocks {
		t.Fatalf("lock-bearing stroke = %#v, want %#v", withLocks, withoutLocks)
	}
	if withoutLocks == hyper {
		t.Fatal("Super and Hyper strokes compare equal")
	}
	if got := withoutLocks.Notation(); got != "Super+Enter" {
		t.Fatalf("Super notation = %q", got)
	}
	if got := hyper.Notation(); got != "Hyper+Enter" {
		t.Fatalf("Hyper notation = %q", got)
	}
	if got := NewFunctionalKeyStroke(57_428, vt.Modifiers{}).Notation(); got != "Functional(57428)" {
		t.Fatalf("Functional notation = %q", got)
	}
}

func TestNodeKeyModifiersDoNotMutateEarlierNodeValues(t *testing.T) {
	descriptor := NewActionDescriptor(
		"app.action",
		"Action",
		[]KeyBinding{NewKeyBinding(NewCharacterKeyStroke('x', vt.Modifiers{}))},
	)
	base := Text[struct{}]("base").OnActions(
		"owner",
		[]Action[struct{}]{NewAction[struct{}](descriptor, nil)},
	)
	scoped := base.WithKeyScope(NewKeyScope("scope", NewKeyMap()))
	replaced := base.OnActions("replacement", nil)

	if base.keyInteraction == nil || len(base.keyInteraction.actions) != 1 || base.keyInteraction.scope != nil {
		t.Fatalf("base key interaction mutated: %#v", base.keyInteraction)
	}
	if scoped.keyInteraction == base.keyInteraction || scoped.keyInteraction.scope == nil {
		t.Fatal("WithKeyScope did not copy the key interaction extension")
	}
	if replaced.keyInteraction == base.keyInteraction || len(replaced.keyInteraction.actions) != 0 {
		t.Fatal("OnActions did not copy the key interaction extension")
	}
}
