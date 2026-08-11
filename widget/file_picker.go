package widget

import (
	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// FilePickerEntry is application-supplied inert filesystem metadata
//
// FilePicker never reads Path or accesses the filesystem itself.
type FilePickerEntry struct {
	// ID is the application-defined stable identity of this entry
	ID tui.NodeID
	// Name is the displayed basename or label
	Name string
	// Path is inert metadata available to the application
	Path string
	// Directory reports whether activation should navigate into the entry
	Directory bool
	// Hidden controls filtering through ShowHidden
	Hidden bool
}

// NewFilePickerFile returns one visible file entry
func NewFilePickerFile(id tui.NodeID, name, path string) FilePickerEntry {
	return FilePickerEntry{ID: id, Name: celltext.NormalizeUTF8(name), Path: celltext.NormalizeUTF8(path)}
}

// NewFilePickerDirectory returns one visible directory entry
func NewFilePickerDirectory(id tui.NodeID, name, path string) FilePickerEntry {
	return FilePickerEntry{
		ID: id, Name: celltext.NormalizeUTF8(name), Path: celltext.NormalizeUTF8(path), Directory: true,
	}
}

// WithHidden replaces whether the entry is hidden by default
func (e FilePickerEntry) WithHidden(hidden bool) FilePickerEntry {
	e.Hidden = hidden
	return e
}

// FilePickerStyle contains the visual styles used by a FilePicker
type FilePickerStyle struct {
	// Normal is used by ordinary files
	Normal vt.Style
	// Directory is merged over directory entries
	Directory vt.Style
	// Hidden is merged over hidden entries when shown
	Hidden vt.Style
	// Selected is merged over the application-selected entry
	Selected vt.Style
	// Focused is merged over the entry that owns focus
	Focused vt.Style
	// Disabled is used when selection changes are unavailable
	Disabled vt.Style
	// Placeholder is used when no entries are visible
	Placeholder vt.Style
}

// DefaultFilePickerStyle returns the standard file picker styles
func DefaultFilePickerStyle() FilePickerStyle {
	return FilePickerStyle{
		Directory: vt.Style{Bold: true}, Hidden: vt.Style{Dim: true},
		Selected: vt.Style{Reverse: true}, Focused: vt.Style{Underline: true},
		Disabled: vt.Style{Dim: true}, Placeholder: vt.Style{Dim: true},
	}
}

const filePickerActionCount = 8

var defaultFilePickerActionDescriptors = [filePickerActionCount]tui.ActionDescriptor{
	tui.NewActionDescriptor(
		ActivateActionID,
		activateActionLabel,
		[]tui.KeyBinding{
			repeatableActionBinding(vt.KeyEnter),
			tui.NewKeyBinding(tui.NewCharacterKeyStroke(' ', vt.Modifiers{})).
				WithRepeatPolicy(tui.RepeatAllow),
			repeatableActionBinding(vt.KeyRight),
		},
	),
	tui.NewActionDescriptor(
		SelectionPreviousActionID,
		selectionPreviousActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyUp)},
	),
	tui.NewActionDescriptor(
		SelectionNextActionID,
		selectionNextActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyDown)},
	),
	tui.NewActionDescriptor(
		SelectionFirstActionID,
		selectionFirstActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyHome)},
	),
	tui.NewActionDescriptor(
		SelectionLastActionID,
		selectionLastActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyEnd)},
	),
	tui.NewActionDescriptor(
		SelectionPreviousPageActionID,
		selectionPreviousPageActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyPageUp)},
	),
	tui.NewActionDescriptor(
		SelectionNextPageActionID,
		selectionNextPageActionLabel,
		[]tui.KeyBinding{repeatableActionBinding(vt.KeyPageDown)},
	),
	tui.NewActionDescriptor(
		NavigationBackActionID,
		navigationBackActionLabel,
		[]tui.KeyBinding{
			repeatableActionBinding(vt.KeyLeft),
			repeatableActionBinding(vt.KeyBackspace),
		},
	),
}

type filePickerAction uint8

const (
	filePickerActivate filePickerAction = iota
	filePickerPrevious
	filePickerNext
	filePickerFirst
	filePickerLast
	filePickerPreviousPage
	filePickerNextPage
	filePickerBack
)

var filePickerActions = [filePickerActionCount]filePickerAction{
	filePickerActivate,
	filePickerPrevious,
	filePickerNext,
	filePickerFirst,
	filePickerLast,
	filePickerPreviousPage,
	filePickerNextPage,
	filePickerBack,
}

// FilePicker is a controlled browser over application-supplied entries
type FilePicker[Message any] struct {
	id             tui.NodeID
	entries        []FilePickerEntry
	selected       int
	viewportHeight int
	showHidden     bool
	placeholder    string
	enabled        bool
	style          FilePickerStyle
	onSelect       func(int) Message
	onOpen         func(int) Message
	onBack         func() Message
}

// NewFilePicker returns a controlled entry browser
//
// A nil onSelect function creates a disabled picker.
func NewFilePicker[Message any](id tui.NodeID, entries []FilePickerEntry, selected int, onSelect func(int) Message) FilePicker[Message] {
	return FilePicker[Message]{
		id: id, entries: cloneFilePickerEntries(entries), selected: selected,
		placeholder: "No entries", enabled: onSelect != nil,
		style: DefaultFilePickerStyle(), onSelect: onSelect,
	}
}

// OnOpen sets the handler receiving an original entry index on activation
func (p FilePicker[Message]) OnOpen(handler func(int) Message) FilePicker[Message] {
	p.onOpen = handler
	return p
}

// OnBack sets the navigation-back handler
//
// Unmodified Left and Backspace are the ordered default bindings.
func (p FilePicker[Message]) OnBack(handler func() Message) FilePicker[Message] {
	p.onBack = handler
	return p
}

// ShowHidden sets whether hidden entries remain visible
func (p FilePicker[Message]) ShowHidden(show bool) FilePicker[Message] {
	p.showHidden = show
	return p
}

// Viewport limits rendering to a selection-following entry window
func (p FilePicker[Message]) Viewport(height int) FilePicker[Message] {
	p.viewportHeight = max(height, 0)
	return p
}

// Placeholder replaces text shown when no entries are visible
func (p FilePicker[Message]) Placeholder(placeholder string) FilePicker[Message] {
	p.placeholder = celltext.NormalizeUTF8(placeholder)
	return p
}

// Enabled sets whether the picker can receive focus and emit messages
func (p FilePicker[Message]) Enabled(enabled bool) FilePicker[Message] {
	p.enabled = enabled && p.onSelect != nil
	return p
}

// Style replaces the file picker styles
func (p FilePicker[Message]) Style(style FilePickerStyle) FilePicker[Message] {
	p.style = style
	return p
}

// ActionDescriptors returns the ordered semantic actions declared by the root
//
// Activation and navigation back are disabled-pass-through when their
// corresponding callbacks are absent.
func (p FilePicker[Message]) ActionDescriptors() []tui.ActionDescriptor {
	descriptors := filePickerActionDescriptors(
		p.enabled && filePickerHasVisibleEntries(p.entries, p.showHidden),
		p.onOpen != nil,
		p.onBack != nil,
	)
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for this file picker
func (p FilePicker[Message]) Node() tui.Node[Message] {
	visible := filePickerVisibleIndices(p.entries, p.showHidden)
	selectedPosition, hasSelection := normalizedListSelection(visible, p.selected)
	descriptors := filePickerActionDescriptors(
		p.enabled && hasSelection,
		p.onOpen != nil,
		p.onBack != nil,
	)
	if !hasSelection {
		return tui.StyledText[Message](p.placeholder, p.style.Placeholder).
			WithID(p.id).
			OnActions(p.id, disabledFilePickerActions[Message](descriptors))
	}
	start, end := 0, len(visible)
	if p.viewportHeight > 0 {
		start, end = treeViewportRange(len(visible), selectedPosition, p.viewportHeight)
	}
	children := make([]tui.Node[Message], 0, end-start)
	for position := start; position < end; position++ {
		originalIndex := visible[position]
		entry := p.entries[originalIndex]
		isSelected := position == selectedPosition
		style := p.style.Normal
		if entry.Directory {
			style = style.Merge(p.style.Directory)
		}
		if entry.Hidden {
			style = style.Merge(p.style.Hidden)
		}
		if isSelected {
			style = style.Merge(p.style.Selected)
		}
		if !p.enabled {
			style = p.style.Disabled
		}
		prefix := "  "
		if entry.Directory {
			prefix = "▸ "
		}
		row := tui.StyledText[Message](prefix+entry.Name, style)
		if !p.enabled {
			children = append(children, row.WithID(entry.ID))
			continue
		}
		if isSelected {
			children = append(children, tui.Column(row.WithID(entry.ID)).
				Focusable(p.id).
				WithFocusedStyle(p.style.Focused).
				OnActions(
					p.id,
					filePickerSemanticActions(
						descriptors,
						visible,
						selectedPosition,
						p.viewportHeight,
						p.id,
						p.onSelect,
						p.onOpen,
						p.onBack,
					),
				).
				OnEvent(p.id, p.selectedPointerHandler(originalIndex)))
			continue
		}
		entryID := entry.ID
		selection := originalIndex
		children = append(children, row.WithID(entryID).OnEvent(entryID, func(event vt.Event) tui.EventResult[Message] {
			if !isPointerActivationEvent(event) {
				return tui.IgnoreResult[Message]()
			}
			result := tui.ConsumeResult[Message]().Focus(p.id).Emit(p.onSelect(selection))
			if p.onOpen != nil {
				result = result.Emit(p.onOpen(selection))
			}
			return result
		}))
	}
	root := tui.Column(children...)
	if p.viewportHeight > 0 {
		root = root.WithLength(tui.Fixed(uint32(p.viewportHeight)))
	}
	if !p.enabled {
		return root.WithID(p.id).OnActions(p.id, disabledFilePickerActions[Message](descriptors))
	}
	return root
}

func (p FilePicker[Message]) selectedPointerHandler(originalIndex int) func(vt.Event) tui.EventResult[Message] {
	return func(event vt.Event) tui.EventResult[Message] {
		if !isPointerActivationEvent(event) {
			return tui.IgnoreResult[Message]()
		}
		result := tui.ConsumeResult[Message]().Focus(p.id)
		if p.onOpen != nil {
			result = result.Emit(p.onOpen(originalIndex))
		}
		return result
	}
}

func filePickerSemanticActions[Message any](
	descriptors [filePickerActionCount]tui.ActionDescriptor,
	visible []int,
	selected int,
	viewportHeight int,
	focusID tui.NodeID,
	onSelect func(int) Message,
	onOpen func(int) Message,
	onBack func() Message,
) []tui.Action[Message] {
	actions := make([]tui.Action[Message], len(descriptors))
	for index, descriptor := range descriptors {
		action := filePickerActions[index]
		actions[index] = tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
			return filePickerActionResult(
				action,
				visible,
				selected,
				viewportHeight,
				focusID,
				onSelect,
				onOpen,
				onBack,
			)
		})
	}
	return actions
}

func disabledFilePickerActions[Message any](
	descriptors [filePickerActionCount]tui.ActionDescriptor,
) []tui.Action[Message] {
	actions := make([]tui.Action[Message], len(descriptors))
	for index, descriptor := range descriptors {
		actions[index] = tui.NewAction[Message](descriptor, nil)
	}
	return actions
}

func filePickerActionResult[Message any](
	action filePickerAction,
	visible []int,
	selected int,
	viewportHeight int,
	focusID tui.NodeID,
	onSelect func(int) Message,
	onOpen func(int) Message,
	onBack func() Message,
) tui.EventResult[Message] {
	if len(visible) == 0 {
		return tui.IgnoreResult[Message]()
	}
	selected = min(max(selected, 0), len(visible)-1)
	result := tui.ConsumeResult[Message]().Focus(focusID)
	switch action {
	case filePickerActivate:
		if onOpen == nil {
			return tui.IgnoreResult[Message]()
		}
		return result.Emit(onOpen(visible[selected]))
	case filePickerBack:
		if onBack == nil {
			return tui.IgnoreResult[Message]()
		}
		return result.Emit(onBack())
	default:
		position, ok := filePickerPositionForAction(selected, len(visible), viewportHeight, action)
		if !ok {
			return tui.IgnoreResult[Message]()
		}
		if position != selected {
			result = result.Emit(onSelect(visible[position]))
		}
		return result
	}
}

func filePickerPositionForAction(
	selected, count, viewportHeight int,
	action filePickerAction,
) (int, bool) {
	if count <= 0 {
		return 0, false
	}
	selected = min(max(selected, 0), count-1)
	switch action {
	case filePickerPrevious:
		return max(selected-1, 0), true
	case filePickerNext:
		return min(selected+1, count-1), true
	case filePickerFirst:
		return 0, true
	case filePickerLast:
		return count - 1, true
	case filePickerPreviousPage, filePickerNextPage:
		step := viewportHeight
		if step <= 0 {
			step = min(count, 10)
		}
		if action == filePickerPreviousPage {
			return max(selected-step, 0), true
		}
		return min(selected+step, count-1), true
	default:
		return 0, false
	}
}

func filePickerActionDescriptors(
	available, hasOpen, hasBack bool,
) [filePickerActionCount]tui.ActionDescriptor {
	descriptors := defaultFilePickerActionDescriptors
	for index, action := range filePickerActions {
		enabled := available
		if action == filePickerActivate {
			enabled = enabled && hasOpen
		} else if action == filePickerBack {
			enabled = enabled && hasBack
		}
		availability := tui.ActionEnabled
		if !enabled {
			availability = tui.ActionDisabledPassThrough
		}
		descriptors[index] = descriptors[index].WithAvailability(availability)
	}
	return descriptors
}

func filePickerHasVisibleEntries(entries []FilePickerEntry, showHidden bool) bool {
	for _, entry := range entries {
		if showHidden || !entry.Hidden {
			return true
		}
	}
	return false
}

func filePickerVisibleIndices(entries []FilePickerEntry, showHidden bool) []int {
	visible := make([]int, 0, len(entries))
	for index, entry := range entries {
		if showHidden || !entry.Hidden {
			visible = append(visible, index)
		}
	}
	return visible
}

func cloneFilePickerEntries(entries []FilePickerEntry) []FilePickerEntry {
	output := append([]FilePickerEntry(nil), entries...)
	for index := range output {
		output[index].Name = celltext.NormalizeUTF8(output[index].Name)
		output[index].Path = celltext.NormalizeUTF8(output[index].Path)
	}
	return output
}
