package tui

import celltext "github.com/mayahiro/nagi-go/text"

type textEditKind uint8

const (
	textEditInsert textEditKind = iota
	textEditPaste
	textEditLeft
	textEditRight
	textEditHome
	textEditEnd
	textEditBackspace
	textEditDelete
)

func applyTextEdit(value string, cursor int, edit textEditKind, inserted string) (string, int) {
	value = celltext.NormalizeUTF8(value)
	inserted = celltext.NormalizeUTF8(inserted)
	cursor = normalizeTextCursor(value, cursor)
	switch edit {
	case textEditInsert, textEditPaste:
		output := value[:cursor] + inserted + value[cursor:]
		intended := cursor + len(inserted)
		for _, boundary := range celltext.GraphemeBoundaries(output) {
			if boundary >= intended {
				return output, boundary
			}
		}
		return output, len(output)
	case textEditLeft:
		if previous, ok := celltext.PreviousGraphemeBoundary(value, cursor); ok {
			return value, previous
		}
		return value, 0
	case textEditRight:
		if next, ok := celltext.NextGraphemeBoundary(value, cursor); ok {
			return value, next
		}
		return value, len(value)
	case textEditHome:
		return value, 0
	case textEditEnd:
		return value, len(value)
	case textEditBackspace:
		start := cursor
		if previous, ok := celltext.PreviousGraphemeBoundary(value, cursor); ok {
			start = previous
		}
		return value[:start] + value[cursor:], start
	case textEditDelete:
		end := cursor
		if next, ok := celltext.NextGraphemeBoundary(value, cursor); ok {
			end = next
		}
		return value[:cursor] + value[end:], cursor
	default:
		panic("nagi-tui: invalid text edit")
	}
}

func normalizeTextCursor(value string, cursor int) int {
	value = celltext.NormalizeUTF8(value)
	cursor = min(max(cursor, 0), len(value))
	result := 0
	for _, boundary := range celltext.GraphemeBoundaries(value) {
		if boundary > cursor {
			break
		}
		result = boundary
	}
	return result
}
