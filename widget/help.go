package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"strings"

	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagitui-go"
)

// HelpMode controls the arrangement of key bindings
type HelpMode uint8

const (
	// HelpCompact renders bindings on one line
	HelpCompact HelpMode = iota
	// HelpFull renders one aligned binding per line
	HelpFull
)

// HelpBinding is one key description shown by Help
type HelpBinding struct {
	// Key is the user-facing key notation
	Key string
	// Description explains the action
	Description string
	// Enabled controls whether the binding is normally shown
	Enabled bool
}

// NewHelpBinding returns an enabled key binding
func NewHelpBinding(key, description string) HelpBinding {
	return HelpBinding{
		Key: celltext.NormalizeUTF8(key), Description: celltext.NormalizeUTF8(description), Enabled: true,
	}
}

// WithEnabled replaces whether the binding is available
func (b HelpBinding) WithEnabled(enabled bool) HelpBinding {
	b.Enabled = enabled
	return b
}

// HelpStyle contains the visual styles used by Help
type HelpStyle struct {
	// Key is used by key notation
	Key vt.Style
	// Description is used by action descriptions
	Description vt.Style
	// Separator is used between compact bindings
	Separator vt.Style
	// Disabled is merged over unavailable bindings when they are shown
	Disabled vt.Style
}

// DefaultHelpStyle returns the standard help styles
func DefaultHelpStyle() HelpStyle {
	return HelpStyle{
		Key: vt.Style{Bold: true}, Description: vt.Style{Dim: true},
		Separator: vt.Style{Dim: true}, Disabled: vt.Style{Dim: true},
	}
}

// Help renders discoverable key bindings in compact or full form
type Help[Message any] struct {
	bindings     []HelpBinding
	mode         HelpMode
	separator    string
	showDisabled bool
	style        HelpStyle
}

// NewHelp returns compact help for the supplied bindings
func NewHelp[Message any](bindings []HelpBinding) Help[Message] {
	return Help[Message]{
		bindings: cloneHelpBindings(bindings), separator: " • ", style: DefaultHelpStyle(),
	}
}

// Mode replaces the binding arrangement
func (h Help[Message]) Mode(mode HelpMode) Help[Message] {
	h.mode = mode
	return h
}

// Separator replaces text between compact bindings
func (h Help[Message]) Separator(separator string) Help[Message] {
	h.separator = celltext.NormalizeUTF8(separator)
	return h
}

// ShowDisabled sets whether unavailable bindings remain visible
func (h Help[Message]) ShowDisabled(show bool) Help[Message] {
	h.showDisabled = show
	return h
}

// Style replaces the help styles
func (h Help[Message]) Style(style HelpStyle) Help[Message] {
	h.style = style
	return h
}

// Node builds the public semantic node for this help view
func (h Help[Message]) Node() tui.Node[Message] {
	bindings := make([]HelpBinding, 0, len(h.bindings))
	for _, binding := range h.bindings {
		if binding.Enabled || h.showDisabled {
			bindings = append(bindings, binding)
		}
	}
	if h.mode == HelpFull {
		return h.fullNode(bindings)
	}
	parts := make([]tui.Node[Message], 0, len(bindings)*5)
	for index, binding := range bindings {
		if index > 0 {
			parts = append(parts, tui.StyledText[Message](h.separator, h.style.Separator))
		}
		keyStyle, descriptionStyle := h.style.Key, h.style.Description
		if !binding.Enabled {
			keyStyle = keyStyle.Merge(h.style.Disabled)
			descriptionStyle = descriptionStyle.Merge(h.style.Disabled)
		}
		parts = append(parts,
			tui.StyledText[Message](binding.Key, keyStyle),
			tui.StyledText[Message](" ", vt.Style{}),
			tui.StyledText[Message](binding.Description, descriptionStyle),
		)
	}
	return tui.Row(parts...)
}

func (h Help[Message]) fullNode(bindings []HelpBinding) tui.Node[Message] {
	keyWidth := 0
	for _, binding := range bindings {
		keyWidth = max(keyWidth, celltext.Width(binding.Key, celltext.ModernWidth()))
	}
	rows := make([]tui.Node[Message], 0, len(bindings))
	for _, binding := range bindings {
		keyStyle, descriptionStyle := h.style.Key, h.style.Description
		if !binding.Enabled {
			keyStyle = keyStyle.Merge(h.style.Disabled)
			descriptionStyle = descriptionStyle.Merge(h.style.Disabled)
		}
		padding := strings.Repeat(" ", keyWidth-celltext.Width(binding.Key, celltext.ModernWidth()))
		rows = append(rows, tui.Row(
			tui.StyledText[Message](binding.Key+padding, keyStyle),
			tui.StyledText[Message]("  ", vt.Style{}),
			tui.StyledText[Message](binding.Description, descriptionStyle),
		))
	}
	return tui.Column(rows...)
}

func cloneHelpBindings(bindings []HelpBinding) []HelpBinding {
	output := append([]HelpBinding(nil), bindings...)
	for index := range output {
		output[index].Key = celltext.NormalizeUTF8(output[index].Key)
		output[index].Description = celltext.NormalizeUTF8(output[index].Description)
	}
	return output
}
