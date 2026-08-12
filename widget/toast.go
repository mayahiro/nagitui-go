package widget

import (
	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

// ToastTone is an application-selected visual tone for one Toast
type ToastTone uint8

const (
	// ToastNeutral represents status without additional severity
	ToastNeutral ToastTone = iota
	// ToastInfo represents informational status
	ToastInfo
	// ToastSuccess represents successful completion
	ToastSuccess
	// ToastWarning represents warning status
	ToastWarning
	// ToastError represents error status
	ToastError
)

// ToastPlacement selects the viewport corner used by a ToastRegion
type ToastPlacement uint8

const (
	// ToastTopEnd places the visible group at the top-right corner
	ToastTopEnd ToastPlacement = iota
	// ToastTopStart places the visible group at the top-left corner
	ToastTopStart
	// ToastBottomEnd places the visible group at the bottom-right corner
	ToastBottomEnd
	// ToastBottomStart places the visible group at the bottom-left corner
	ToastBottomStart
)

// ToastStyle contains replaceable visual styles used by a ToastRegion
type ToastStyle struct {
	// Neutral is the border style for neutral status
	Neutral vt.Style
	// Info is the border style for informational status
	Info vt.Style
	// Success is the border style for successful completion
	Success vt.Style
	// Warning is the border style for warnings
	Warning vt.Style
	// Error is the border style for errors
	Error vt.Style
	// Dismiss is used by an optional focusable dismissal button
	Dismiss ButtonStyle
}

// DefaultToastStyle returns attribute-only tone styles and the standard button style
func DefaultToastStyle() ToastStyle {
	return ToastStyle{
		Info:    vt.Style{Bold: true},
		Success: vt.Style{Underline: true},
		Warning: vt.Style{Bold: true, Underline: true},
		Error:   vt.Style{Reverse: true},
		Dismiss: DefaultButtonStyle(),
	}
}

type toastDismiss[Message any] struct {
	id      tui.NodeID
	handler func() Message
}

// Toast is one controlled application notification with a lazily constructed body
//
// Toast owns no timeout or lifecycle state. Applications remove records from
// their controlled collection and may use an After Effect with the Toast ID or
// an application generation to ignore stale completion messages.
type Toast[Message any] struct {
	id         tui.NodeID
	tone       ToastTone
	body       func() tui.Node[Message]
	dismiss    toastDismiss[Message]
	hasDismiss bool
}

// NewToast returns a neutral Toast with one lazy body builder
//
// A nil body builder produces an empty body.
func NewToast[Message any](id tui.NodeID, body func() tui.Node[Message]) Toast[Message] {
	return Toast[Message]{id: id, body: body}
}

// Tone sets the application-defined visual tone
//
// Unknown values use neutral.
func (t Toast[Message]) Tone(tone ToastTone) Toast[Message] {
	t.tone = normalizedToastTone(tone)
	return t
}

// OnDismiss adds a focusable dismissal button with an explicit stable Node ID
//
// A nil handler removes the dismissal button.
func (t Toast[Message]) OnDismiss(buttonID tui.NodeID, handler func() Message) Toast[Message] {
	t.dismiss = toastDismiss[Message]{id: buttonID, handler: handler}
	t.hasDismiss = handler != nil
	return t
}

// ID returns the stable Toast root ID
func (t Toast[Message]) ID() tui.NodeID {
	return t.id
}

// ConfiguredTone returns the configured visual tone
func (t Toast[Message]) ConfiguredTone() ToastTone {
	return normalizedToastTone(t.tone)
}

func (t Toast[Message]) node(style ToastStyle) tui.Node[Message] {
	body := tui.Column[Message]()
	if t.body != nil {
		body = t.body()
	}
	content := body
	if t.hasDismiss {
		content = tui.Row(
			body.WithLength(tui.Flex(1)),
			tui.Gap[Message](1),
			NewButton(t.dismiss.id, "x", t.dismiss.handler).Style(style.Dismiss).Node(),
		)
	}
	return tui.Border(content, toastToneStyle(style, t.tone)).WithID(t.id)
}

// ToastRegion is a corner-aligned overlay for a controlled sequence of Toast values
//
// Input order is oldest to newest. Only the newest visible-limit bodies are
// constructed, and retained values preserve source order from top to bottom.
type ToastRegion[Message any] struct {
	base         tui.Node[Message]
	toasts       []Toast[Message]
	placement    ToastPlacement
	visibleLimit int
	gap          uint32
	style        ToastStyle
}

// NewToastRegion returns a top-end region showing at most three Toasts
//
// The constructor copies the supplied Toast slice.
func NewToastRegion[Message any](base tui.Node[Message], toasts []Toast[Message]) ToastRegion[Message] {
	return ToastRegion[Message]{
		base: base, toasts: append([]Toast[Message](nil), toasts...),
		placement: ToastTopEnd, visibleLimit: 3, gap: 1, style: DefaultToastStyle(),
	}
}

// Placement sets the viewport corner used by the visible group
//
// Unknown values use top-end.
func (r ToastRegion[Message]) Placement(placement ToastPlacement) ToastRegion[Message] {
	r.placement = normalizedToastPlacement(placement)
	return r
}

// VisibleLimit sets the greatest number of Toast bodies constructed and displayed
//
// Negative values use zero.
func (r ToastRegion[Message]) VisibleLimit(limit int) ToastRegion[Message] {
	r.visibleLimit = max(limit, 0)
	return r
}

// Gap sets the empty rows inserted between visible Toasts
func (r ToastRegion[Message]) Gap(gap uint32) ToastRegion[Message] {
	r.gap = gap
	return r
}

// Style replaces all visual styles used by the region
func (r ToastRegion[Message]) Style(style ToastStyle) ToastRegion[Message] {
	r.style = style
	return r
}

// Node builds the public semantic overlay without constructing omitted bodies
func (r ToastRegion[Message]) Node() tui.Node[Message] {
	visible := min(r.visibleLimit, len(r.toasts))
	if visible == 0 {
		return r.base
	}
	first := len(r.toasts) - visible
	children := make([]tui.Node[Message], 0, visible*2-1)
	for index := first; index < len(r.toasts); index++ {
		if index != first {
			children = append(children, tui.Gap[Message](r.gap))
		}
		children = append(children, r.toasts[index].node(r.style))
	}
	horizontal, vertical := tui.AlignEnd, tui.AlignTop
	switch normalizedToastPlacement(r.placement) {
	case ToastTopStart:
		horizontal = tui.AlignStart
	case ToastBottomEnd:
		vertical = tui.AlignBottom
	case ToastBottomStart:
		horizontal, vertical = tui.AlignStart, tui.AlignBottom
	}
	layer := tui.Align(tui.Column(children...), horizontal, vertical)
	return tui.Overlay(r.base, layer)
}

func normalizedToastTone(tone ToastTone) ToastTone {
	if tone <= ToastError {
		return tone
	}
	return ToastNeutral
}

func normalizedToastPlacement(placement ToastPlacement) ToastPlacement {
	if placement <= ToastBottomStart {
		return placement
	}
	return ToastTopEnd
}

func toastToneStyle(style ToastStyle, tone ToastTone) vt.Style {
	switch normalizedToastTone(tone) {
	case ToastInfo:
		return style.Info
	case ToastSuccess:
		return style.Success
	case ToastWarning:
		return style.Warning
	case ToastError:
		return style.Error
	default:
		return style.Neutral
	}
}
