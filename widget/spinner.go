package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

var spinnerFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SpinnerFrameCount is the number of stable frames in the spinner cycle
const SpinnerFrameCount = len(spinnerFrames)

// SpinnerStyle contains the visual style used by a Spinner
type SpinnerStyle struct {
	// Content is used by the frame and optional label
	Content vt.Style
}

// DefaultSpinnerStyle returns the standard spinner style
func DefaultSpinnerStyle() SpinnerStyle {
	return SpinnerStyle{Content: vt.Style{Bold: true}}
}

// Spinner is a pure view driven by an application-owned tick
type Spinner[Message any] struct {
	tick  uint64
	label string
	style SpinnerStyle
}

// NewSpinner returns an unlabeled spinner
func NewSpinner[Message any](tick uint64) Spinner[Message] {
	return Spinner[Message]{tick: tick, style: DefaultSpinnerStyle()}
}

// Label sets text displayed after the spinner frame
func (s Spinner[Message]) Label(label string) Spinner[Message] {
	s.label = label
	return s
}

// Style replaces the spinner style
func (s Spinner[Message]) Style(style SpinnerStyle) Spinner[Message] {
	s.style = style
	return s
}

// Node builds the public semantic node for this spinner
func (s Spinner[Message]) Node() tui.Node[Message] {
	return tui.StyledText[Message](renderedSpinner(s.tick, s.label), s.style.Content)
}

func renderedSpinner(tick uint64, label string) string {
	frame := spinnerFrames[tick%uint64(len(spinnerFrames))]
	if label == "" {
		return frame
	}
	return frame + " " + label
}
