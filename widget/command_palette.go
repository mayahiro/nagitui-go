package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// Command is one searchable entry rendered by a CommandPalette
type Command struct {
	// ID is the application-defined stable identity of this command
	ID tui.NodeID
	// Label is the displayed command text
	Label string
	// Keywords contains additional search terms
	Keywords []string
}

// NewCommand returns a command with a stable identity
func NewCommand(id tui.NodeID, label string) Command {
	return Command{ID: id, Label: label}
}

// WithKeywords replaces terms considered during ASCII case-insensitive filtering
func (c Command) WithKeywords(keywords ...string) Command {
	c.Keywords = append([]string(nil), keywords...)
	return c
}

// CommandPaletteStyle contains the visual styles used by a CommandPalette
type CommandPaletteStyle struct {
	// Border is used by the outer border
	Border vt.Style
	// Title is used by an optional title
	Title vt.Style
	// Query is used by query text
	Query vt.Style
	// Placeholder is used by query placeholder text
	Placeholder vt.Style
	// Normal is used by unselected commands
	Normal vt.Style
	// Selected is used by the application-selected command
	Selected vt.Style
	// Focused is merged over the command or input that owns focus
	Focused vt.Style
	// Empty is used by the empty result notice
	Empty vt.Style
	// Disabled is used by query and command text while disabled
	Disabled vt.Style
}

// DefaultCommandPaletteStyle returns the standard command palette styles
func DefaultCommandPaletteStyle() CommandPaletteStyle {
	return CommandPaletteStyle{
		Title: vt.Style{Bold: true}, Placeholder: vt.Style{Dim: true},
		Selected: vt.Style{Reverse: true}, Focused: vt.Style{Underline: true},
		Empty: vt.Style{Dim: true}, Disabled: vt.Style{Dim: true},
	}
}

// CommandPalette is a searchable command chooser with application-owned query and selection
type CommandPalette[Message any] struct {
	id          tui.NodeID
	inputID     tui.NodeID
	query       string
	commands    []Command
	selected    int
	enabled     bool
	title       string
	placeholder string
	emptyLabel  string
	style       CommandPaletteStyle
	onQuery     func(string) Message
	onSelect    func(int) Message
	onActivate  func(int) Message
}

// NewCommandPalette returns a palette using original command indices for selection
//
// A nil callback creates a disabled palette
func NewCommandPalette[Message any](
	id, inputID tui.NodeID,
	query string,
	commands []Command,
	selected int,
	onQuery func(string) Message,
	onSelect, onActivate func(int) Message,
) CommandPalette[Message] {
	return CommandPalette[Message]{
		id: id, inputID: inputID, query: query, commands: cloneCommands(commands), selected: selected,
		enabled:     onQuery != nil && onSelect != nil && onActivate != nil,
		placeholder: "Type to filter", emptyLabel: "No matching commands",
		style: DefaultCommandPaletteStyle(), onQuery: onQuery, onSelect: onSelect, onActivate: onActivate,
	}
}

// Enabled sets whether the input and command rows can receive focus and emit messages
func (p CommandPalette[Message]) Enabled(enabled bool) CommandPalette[Message] {
	p.enabled = enabled && p.onQuery != nil && p.onSelect != nil && p.onActivate != nil
	return p
}

// Title sets an optional title rendered above the query
func (p CommandPalette[Message]) Title(title string) CommandPalette[Message] {
	p.title = title
	return p
}

// Placeholder sets query placeholder text
func (p CommandPalette[Message]) Placeholder(placeholder string) CommandPalette[Message] {
	p.placeholder = placeholder
	return p
}

// EmptyLabel sets the notice rendered when no command matches
func (p CommandPalette[Message]) EmptyLabel(label string) CommandPalette[Message] {
	p.emptyLabel = label
	return p
}

// Style replaces the command palette styles
func (p CommandPalette[Message]) Style(style CommandPaletteStyle) CommandPalette[Message] {
	p.style = style
	return p
}

// CommandActionDescriptor returns the semantic activation action declared by each command row
//
// The descriptor is disabled-pass-through when the palette is disabled or its
// current filter has no matching command.
func (p CommandPalette[Message]) CommandActionDescriptor() tui.ActionDescriptor {
	return commandPaletteCommandActionDescriptor(p.enabled && hasVisibleCommand(p.commands, p.query))
}

// ActionDescriptors returns the ordered semantic actions declared by the palette root
//
// The order is activate, previous, next, first, and last. Every descriptor is
// disabled-pass-through when the palette is disabled or its current filter has
// no matching command.
func (p CommandPalette[Message]) ActionDescriptors() []tui.ActionDescriptor {
	descriptors := commandPaletteActionDescriptors(p.enabled && hasVisibleCommand(p.commands, p.query))
	return append([]tui.ActionDescriptor(nil), descriptors[:]...)
}

// Node builds the public semantic node for this command palette
func (p CommandPalette[Message]) Node() tui.Node[Message] {
	visible := filteredCommandIndices(p.commands, p.query)
	selectedPosition, hasSelection := normalizedCommandSelection(visible, p.selected)
	actionsEnabled := p.enabled && hasSelection
	commandDescriptor := commandPaletteCommandActionDescriptor(actionsEnabled)
	actionDescriptors := commandPaletteActionDescriptors(actionsEnabled)
	visibleIDs := make([]tui.NodeID, len(visible))
	children := make([]tui.Node[Message], 0, len(visible)+2)
	if p.title != "" {
		children = append(children, tui.StyledText[Message](p.title, p.style.Title))
	}
	if p.enabled {
		children = append(children, tui.StyledTextInput(
			p.inputID, p.query, p.placeholder, p.style.Query, p.style.Placeholder, p.onQuery,
		).WithFocusedStyle(p.style.Focused))
	} else {
		query := p.query
		if query == "" {
			query = p.placeholder
		}
		children = append(children, tui.StyledText[Message]("> "+query, p.style.Disabled))
	}

	if len(visible) == 0 {
		children = append(children, tui.StyledText[Message](p.emptyLabel, p.style.Empty))
	} else {
		for position, originalIndex := range visible {
			command := p.commands[originalIndex]
			visibleIDs[position] = command.ID
			isSelected := hasSelection && position == selectedPosition
			marker := "  "
			if isSelected {
				marker = "> "
			}
			style := p.style.Normal
			if isSelected {
				style = p.style.Selected
			}
			if !p.enabled {
				style = p.style.Disabled
			}
			node := tui.StyledText[Message](marker+command.Label, style)
			if !p.enabled {
				children = append(children, node.WithID(command.ID).OnActions(
					command.ID,
					[]tui.Action[Message]{tui.NewAction[Message](commandDescriptor, nil)},
				))
				continue
			}
			selection := originalIndex
			commandID := command.ID
			children = append(children, node.
				Focusable(commandID).
				WithFocusedStyle(p.style.Focused).
				OnActions(commandID, []tui.Action[Message]{
					tui.NewAction(commandDescriptor, func(tui.ActionEvent) tui.EventResult[Message] {
						return commandActivationResult(
							isSelected, selection, commandID, p.onSelect, p.onActivate,
						)
					}),
				}).
				OnEvent(commandID, func(event vt.Event) tui.EventResult[Message] {
					if !isPointerActivationEvent(event) {
						return tui.IgnoreResult[Message]()
					}
					return commandActivationResult(
						isSelected, selection, commandID, p.onSelect, p.onActivate,
					)
				}))
		}
	}

	root := tui.Border(tui.Column(children...), p.style.Border).WithID(p.id)
	if !p.enabled || !hasSelection {
		return root.OnActions(p.id, disabledCollectionActions[Message](actionDescriptors))
	}
	actions := make([]tui.Action[Message], len(actionDescriptors))
	for index, descriptor := range actionDescriptors {
		action := collectionAction(index)
		actions[index] = tui.NewAction(descriptor, func(tui.ActionEvent) tui.EventResult[Message] {
			return commandPaletteActionResult(
				action, selectedPosition, visible, visibleIDs, p.onSelect, p.onActivate,
			)
		})
	}
	return root.OnActions(p.id, actions)
}

func commandPaletteCommandActionDescriptor(enabled bool) tui.ActionDescriptor {
	return ActivateActionDescriptor().WithAvailability(commandPaletteActionAvailability(enabled))
}

func commandPaletteActionDescriptors(enabled bool) [5]tui.ActionDescriptor {
	return verticalCollectionActionDescriptors(enabled)
}

func commandPaletteActionAvailability(enabled bool) tui.ActionAvailability {
	if enabled {
		return tui.ActionEnabled
	}
	return tui.ActionDisabledPassThrough
}

func commandActivationResult[Message any](
	isSelected bool,
	originalIndex int,
	focusID tui.NodeID,
	onSelect, onActivate func(int) Message,
) tui.EventResult[Message] {
	result := tui.ConsumeResult[Message]().Focus(focusID)
	if !isSelected {
		result = result.Emit(onSelect(originalIndex))
	}
	return result.Emit(onActivate(originalIndex))
}

func commandPaletteActionResult[Message any](
	action collectionAction,
	selectedPosition int,
	visible []int,
	visibleIDs []tui.NodeID,
	onSelect, onActivate func(int) Message,
) tui.EventResult[Message] {
	navigation, navigates := collectionNavigation(action)
	if !navigates {
		return tui.MessageResult(onActivate(visible[selectedPosition]))
	}
	next, _ := navigateSelection(len(visible), selectedPosition, navigation)
	result := tui.ConsumeResult[Message]().Focus(visibleIDs[next])
	if next != selectedPosition {
		result = result.Emit(onSelect(visible[next]))
	}
	return result
}

func cloneCommands(commands []Command) []Command {
	cloned := make([]Command, len(commands))
	for index, command := range commands {
		cloned[index] = NewCommand(command.ID, command.Label).WithKeywords(command.Keywords...)
	}
	return cloned
}

func filteredCommandIndices(commands []Command, query string) []int {
	visible := make([]int, 0, len(commands))
	for index, command := range commands {
		if commandMatches(command, query) {
			visible = append(visible, index)
		}
	}
	return visible
}

func hasVisibleCommand(commands []Command, query string) bool {
	for _, command := range commands {
		if commandMatches(command, query) {
			return true
		}
	}
	return false
}

func commandMatches(command Command, query string) bool {
	if query == "" || containsASCIICaseInsensitive(command.Label, query) {
		return true
	}
	for _, keyword := range command.Keywords {
		if containsASCIICaseInsensitive(keyword, query) {
			return true
		}
	}
	return false
}

func containsASCIICaseInsensitive(value, query string) bool {
	if len(query) > len(value) {
		return false
	}
	for start := 0; start <= len(value)-len(query); start++ {
		matches := true
		for offset := range len(query) {
			left := value[start+offset]
			right := query[offset]
			if left >= 'A' && left <= 'Z' {
				left += 'a' - 'A'
			}
			if right >= 'A' && right <= 'Z' {
				right += 'a' - 'A'
			}
			if left != right {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func normalizedCommandSelection(visible []int, selected int) (int, bool) {
	if len(visible) == 0 {
		return 0, false
	}
	selected = max(selected, 0)
	position := 0
	for index, original := range visible {
		if original > selected {
			break
		}
		position = index
	}
	return position, true
}
