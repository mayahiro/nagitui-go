package widget

import "testing"

func TestTextAreaHistorySkipsNavigationState(t *testing.T) {
	history := NewTextAreaHistory(NewTextAreaState("ab", 0))
	history.Record(NewTextAreaState("ab", 1))
	if _, ok := history.Undo(); ok {
		t.Fatal("Undo succeeded for a cursor-only change")
	}
}

func TestTextAreaHistoryClearsRedoAfterNewContent(t *testing.T) {
	history := NewTextAreaHistory(NewTextAreaStateAtEnd("a"))
	history.Record(NewTextAreaStateAtEnd("ab"))
	if state, ok := history.Undo(); !ok || state.Value() != "a" {
		t.Fatalf("Undo() = %#v, %t", state, ok)
	}
	history.Record(NewTextAreaStateAtEnd("ac"))
	if _, ok := history.Redo(); ok {
		t.Fatal("Redo succeeded after a divergent edit")
	}
}

func TestTextAreaHistoryHonorsLimit(t *testing.T) {
	history := NewTextAreaHistoryWithLimit(NewTextAreaStateAtEnd("a"), 1)
	history.Record(NewTextAreaStateAtEnd("ab"))
	history.Record(NewTextAreaStateAtEnd("abc"))
	if state, ok := history.Undo(); !ok || state.Value() != "ab" {
		t.Fatalf("Undo() = %#v, %t", state, ok)
	}
	if _, ok := history.Undo(); ok {
		t.Fatal("Undo retained more states than its limit")
	}
}
