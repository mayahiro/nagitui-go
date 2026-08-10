package tui

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/internal/conformance"
)

func TestKeyStrokesMatchSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"interaction/key-stroke.txt", "key-stroke",
		"event", "stroke", "repeat", "matches", "notation",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			binding := NewKeyBinding(keymapFixtureStroke(t, record.Field("stroke"))).
				WithRepeatPolicy(keymapFixtureRepeatPolicy(t, record.Field("repeat")))
			if actual, expected := binding.Matches(keymapFixtureEvent(t, record.Field("event"))), keymapFixtureBool(t, record.Field("matches")); actual != expected {
				t.Fatalf("Matches = %t, want %t", actual, expected)
			}
			if actual, expected := binding.Stroke().Notation(), record.Field("notation"); actual != expected {
				t.Fatalf("Notation = %q, want %q", actual, expected)
			}
		})
	}
}

func TestKeyScopeResolutionMatchesSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"interaction/key-scope.txt", "key-scope",
		"action", "label", "defaults", "layers", "availability", "visible",
		"expected-bindings", "expected-scopes",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			actionID := NewActionID(record.Field("action"))
			descriptor := NewActionDescriptor(
				actionID,
				record.Field("label"),
				keymapFixtureBindings(t, record.Field("defaults")),
			).
				WithAvailability(keymapFixtureAvailability(t, record.Field("availability"))).
				WithHelpVisible(keymapFixtureBool(t, record.Field("visible")))
			resolved, err := ResolveActions(
				NewNodeID("owner"),
				[]ActionDescriptor{descriptor},
				keymapFixtureScopes(t, record.Field("layers"), actionID),
			)
			if err != nil {
				t.Fatal(err)
			}
			actions := resolved.Actions()
			if len(actions) != 1 {
				t.Fatalf("actions = %d", len(actions))
			}
			action := actions[0]
			if action.ID() != actionID || action.Label() != record.Field("label") {
				t.Fatalf("action = %s %q", action.ID(), action.Label())
			}
			if actual, expected := action.Bindings(), keymapFixtureBindings(t, record.Field("expected-bindings")); !slices.Equal(actual, expected) {
				t.Fatalf("bindings = %#v, want %#v", actual, expected)
			}
			if actual, expected := action.Availability(), keymapFixtureAvailability(t, record.Field("availability")); actual != expected {
				t.Fatalf("availability = %d, want %d", actual, expected)
			}
			if actual, expected := action.HelpVisible(), keymapFixtureBool(t, record.Field("visible")); actual != expected {
				t.Fatalf("HelpVisible = %t, want %t", actual, expected)
			}
			expectedHelp := 0
			if keymapFixtureBool(t, record.Field("visible")) {
				expectedHelp = 1
			}
			if actual := len(resolved.HelpActions()); actual != expectedHelp {
				t.Fatalf("HelpActions = %d, want %d", actual, expectedHelp)
			}
			if actual, expected := resolved.ScopePath(), keymapFixtureNodeIDs(record.Field("expected-scopes")); !slices.Equal(actual, expected) {
				t.Fatalf("scope path = %v, want %v", actual, expected)
			}
		})
	}
}

func TestKeyConflictsMatchSharedFixtures(t *testing.T) {
	records, err := conformance.Load(
		"interaction/key-conflict.txt", "key-conflict",
		"actions", "scopes", "expected-kind", "expected-actions", "expected-stroke",
	)
	if errors.Is(err, conformance.ErrNoFixtureRoot) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		t.Run(record.ID, func(t *testing.T) {
			owner := NewNodeID("owner")
			scopes := keymapFixtureEmptyScopes(record.Field("scopes"))
			resolved, err := ResolveActions(
				owner,
				keymapFixtureActions(t, record.Field("actions")),
				scopes,
			)
			if record.Field("expected-kind") == "none" {
				if err != nil {
					t.Fatal(err)
				}
				if resolved.Owner() != owner || !slices.Equal(resolved.ScopePath(), keymapFixtureScopeIDs(scopes)) {
					t.Fatalf("resolution identity = %s %v", resolved.Owner(), resolved.ScopePath())
				}
				return
			}
			var conflict *BindingConflictError
			if !errors.As(err, &conflict) {
				t.Fatalf("error = %T %v", err, err)
			}
			if actual, expected := conflict.Kind(), keymapFixtureConflictKind(t, record.Field("expected-kind")); actual != expected {
				t.Fatalf("kind = %d, want %d", actual, expected)
			}
			if conflict.Owner() != owner || !slices.Equal(conflict.ScopePath(), keymapFixtureScopeIDs(scopes)) {
				t.Fatalf("conflict identity = %s %v", conflict.Owner(), conflict.ScopePath())
			}
			if actual, expected := conflict.Actions(), keymapFixtureActionIDs(record.Field("expected-actions")); !slices.Equal(actual, expected) {
				t.Fatalf("actions = %v, want %v", actual, expected)
			}
			actualStroke, hasStroke := conflict.Stroke()
			if record.Field("expected-stroke") == "-" {
				if hasStroke {
					t.Fatalf("stroke = %#v", actualStroke)
				}
			} else {
				expectedStroke := keymapFixtureStroke(t, record.Field("expected-stroke"))
				if !hasStroke || actualStroke != expectedStroke {
					t.Fatalf("stroke = %#v, %t, want %#v", actualStroke, hasStroke, expectedStroke)
				}
			}
		})
	}
}

func keymapFixtureEvent(t *testing.T, value string) vt.Event {
	t.Helper()
	if scalars, ok := strings.CutPrefix(value, "text/"); ok {
		return vt.Event{Kind: vt.EventText, Text: keymapFixtureScalarText(t, scalars)}
	}
	if scalars, ok := strings.CutPrefix(value, "paste/"); ok {
		return vt.Event{Kind: vt.EventPaste, Text: keymapFixtureScalarText(t, scalars)}
	}
	if value == "focus-in" {
		return vt.Event{Kind: vt.EventFocusIn}
	}
	parts := strings.Split(value, "/")
	if len(parts) != 4 || parts[0] != "key" {
		t.Fatalf("invalid event %q", value)
	}
	stroke := keymapFixtureStroke(t, parts[1]+":"+parts[2])
	key := vt.KeyEvent{
		Code: stroke.Code(), Modifiers: stroke.Modifiers(),
		Action: keymapFixtureKeyAction(t, parts[3]), Protocol: vt.KeyProtocolLegacy,
	}
	if character, ok := stroke.Character(); ok {
		key.Character = character
	}
	if function, ok := stroke.Function(); ok {
		key.Function = function
	}
	return vt.Event{Kind: vt.EventKey, Key: key}
}

func keymapFixtureStroke(t *testing.T, value string) KeyStroke {
	t.Helper()
	key, modifierText, ok := strings.Cut(value, ":")
	if !ok {
		t.Fatalf("invalid stroke %q", value)
	}
	modifiers := keymapFixtureModifiers(t, modifierText)
	if scalar, ok := strings.CutPrefix(key, "char-U+"); ok {
		return NewCharacterKeyStroke(keymapFixtureScalar(t, scalar), modifiers)
	}
	if number, ok := strings.CutPrefix(key, "f-"); ok {
		parsed, err := strconv.ParseUint(number, 10, 8)
		if err != nil {
			t.Fatalf("invalid function key %q", key)
		}
		return NewFunctionKeyStroke(uint8(parsed), modifiers)
	}
	var code vt.KeyCode
	switch key {
	case "enter":
		code = vt.KeyEnter
	case "tab":
		code = vt.KeyTab
	case "backspace":
		code = vt.KeyBackspace
	case "escape":
		code = vt.KeyEscape
	case "up":
		code = vt.KeyUp
	case "down":
		code = vt.KeyDown
	case "right":
		code = vt.KeyRight
	case "left":
		code = vt.KeyLeft
	case "home":
		code = vt.KeyHome
	case "end":
		code = vt.KeyEnd
	case "insert":
		code = vt.KeyInsert
	case "delete":
		code = vt.KeyDelete
	case "page-up":
		code = vt.KeyPageUp
	case "page-down":
		code = vt.KeyPageDown
	case "unknown":
		code = vt.KeyUnknown
	default:
		t.Fatalf("invalid key %q", key)
	}
	return NewKeyStroke(code, modifiers)
}

func keymapFixtureModifiers(t *testing.T, value string) vt.Modifiers {
	t.Helper()
	var result vt.Modifiers
	if value == "-" {
		return result
	}
	for _, modifier := range strings.Split(value, "+") {
		switch modifier {
		case "shift":
			result.Shift = true
		case "alt":
			result.Alt = true
		case "control":
			result.Control = true
		case "meta":
			result.Meta = true
		default:
			t.Fatalf("invalid modifier %q", modifier)
		}
	}
	return result
}

func keymapFixtureKeyAction(t *testing.T, value string) vt.KeyAction {
	t.Helper()
	switch value {
	case "unknown":
		return vt.KeyActionUnknown
	case "press":
		return vt.KeyPress
	case "repeat":
		return vt.KeyRepeat
	case "release":
		return vt.KeyRelease
	default:
		t.Fatalf("invalid key action %q", value)
		return vt.KeyActionUnknown
	}
}

func keymapFixtureScalarText(t *testing.T, value string) string {
	t.Helper()
	value, ok := strings.CutPrefix(value, "U+")
	if !ok {
		t.Fatalf("invalid scalar sequence %q", value)
	}
	var output strings.Builder
	for _, scalar := range strings.Split(value, "+U+") {
		output.WriteRune(keymapFixtureScalar(t, scalar))
	}
	return output.String()
}

func keymapFixtureScalar(t *testing.T, value string) rune {
	t.Helper()
	scalar, err := strconv.ParseUint(value, 16, 32)
	if err != nil || !utf8.ValidRune(rune(scalar)) {
		t.Fatalf("invalid scalar %q", value)
	}
	return rune(scalar)
}

func keymapFixtureBindings(t *testing.T, value string) []KeyBinding {
	t.Helper()
	if value == "none" {
		return nil
	}
	parts := strings.Split(value, "/")
	bindings := make([]KeyBinding, len(parts))
	for index, part := range parts {
		fields := strings.Split(part, "@")
		if len(fields) != 3 {
			t.Fatalf("invalid binding %q", part)
		}
		bindings[index] = NewKeyBinding(keymapFixtureStroke(t, fields[0])).
			WithRepeatPolicy(keymapFixtureRepeatPolicy(t, fields[1])).
			WithSupport(keymapFixtureSupport(t, fields[2]))
	}
	return bindings
}

func keymapFixtureRepeatPolicy(t *testing.T, value string) RepeatPolicy {
	t.Helper()
	switch value {
	case "initial":
		return RepeatInitialOnly
	case "allow":
		return RepeatAllow
	default:
		t.Fatalf("invalid repeat policy %q", value)
		return RepeatInitialOnly
	}
}

func keymapFixtureSupport(t *testing.T, value string) BindingSupport {
	t.Helper()
	switch value {
	case "unknown":
		return BindingSupportUnknown
	case "supported":
		return BindingSupported
	case "unsupported":
		return BindingUnsupported
	default:
		t.Fatalf("invalid support %q", value)
		return BindingSupportUnknown
	}
}

func keymapFixtureAvailability(t *testing.T, value string) ActionAvailability {
	t.Helper()
	switch value {
	case "enabled":
		return ActionEnabled
	case "disabled-pass-through":
		return ActionDisabledPassThrough
	case "disabled-consume":
		return ActionDisabledConsume
	default:
		t.Fatalf("invalid availability %q", value)
		return ActionEnabled
	}
}

func keymapFixtureScopes(t *testing.T, value string, action ActionID) []KeyScope {
	t.Helper()
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ";")
	scopes := make([]KeyScope, len(parts))
	for index, part := range parts {
		id, replacement, ok := strings.Cut(part, ":")
		if !ok {
			t.Fatalf("invalid scope %q", part)
		}
		keyMap := NewKeyMap()
		if replacement != "inherit" {
			var err error
			keyMap, err = keyMap.Rebind(action, keymapFixtureBindings(t, replacement))
			if err != nil {
				t.Fatal(err)
			}
		}
		scopes[index] = NewKeyScope(NewNodeID(id), keyMap)
	}
	return scopes
}

func keymapFixtureEmptyScopes(value string) []KeyScope {
	ids := keymapFixtureNodeIDs(value)
	scopes := make([]KeyScope, len(ids))
	for index, id := range ids {
		scopes[index] = NewKeyScope(id, NewKeyMap())
	}
	return scopes
}

func keymapFixtureScopeIDs(scopes []KeyScope) []NodeID {
	ids := make([]NodeID, len(scopes))
	for index, scope := range scopes {
		ids[index] = scope.ID()
	}
	return ids
}

func keymapFixtureActions(t *testing.T, value string) []ActionDescriptor {
	t.Helper()
	parts := strings.Split(value, ";")
	actions := make([]ActionDescriptor, len(parts))
	for index, part := range parts {
		fields := strings.SplitN(part, "@", 3)
		if len(fields) != 3 {
			t.Fatalf("invalid action %q", part)
		}
		var bindings []KeyBinding
		if fields[2] != "none" {
			for _, strokeValue := range strings.Split(fields[2], "/") {
				bindings = append(bindings, NewKeyBinding(keymapFixtureStroke(t, strokeValue)))
			}
		}
		actions[index] = NewActionDescriptor(NewActionID(fields[0]), fields[0], bindings).
			WithAvailability(keymapFixtureAvailability(t, fields[1]))
	}
	return actions
}

func keymapFixtureConflictKind(t *testing.T, value string) BindingConflictKind {
	t.Helper()
	switch value {
	case "duplicate-action":
		return ConflictDuplicateAction
	case "duplicate-binding":
		return ConflictDuplicateBinding
	case "ambiguous-binding":
		return ConflictAmbiguousBinding
	default:
		t.Fatalf("invalid conflict kind %q", value)
		return ConflictDuplicateAction
	}
}

func keymapFixtureBool(t *testing.T, value string) bool {
	t.Helper()
	switch value {
	case "true":
		return true
	case "false":
		return false
	default:
		t.Fatalf("invalid Boolean %q", value)
		return false
	}
}

func keymapFixtureNodeIDs(value string) []NodeID {
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	ids := make([]NodeID, len(parts))
	for index, part := range parts {
		ids[index] = NewNodeID(part)
	}
	return ids
}

func keymapFixtureActionIDs(value string) []ActionID {
	if value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	ids := make([]ActionID, len(parts))
	for index, part := range parts {
		ids[index] = NewActionID(part)
	}
	return ids
}
