package widget

import (
	celltext "github.com/mayahiro/nagi-go/text"
	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// DialogStyle contains the visual styles used by a Dialog
type DialogStyle struct {
	// Border is used by the dialog panel border
	Border vt.Style
}

// DefaultDialogStyle returns the standard dialog styles
func DefaultDialogStyle() DialogStyle {
	return DialogStyle{}
}

// DialogAction is one application-defined command presented by a Dialog
//
// Dialog action labels are single-line text. The stable ID owns focus and
// identifies this action when a dialog selects its default or cancel target.
type DialogAction[Message any] struct {
	id         tui.NodeID
	label      string
	enabled    bool
	style      ButtonStyle
	onActivate func() Message
}

// NewDialogAction returns a dialog action with the standard button styles
//
// A nil onActivate function creates a disabled action.
func NewDialogAction[Message any](
	id tui.NodeID,
	label string,
	onActivate func() Message,
) DialogAction[Message] {
	return DialogAction[Message]{
		id: id, label: label, enabled: onActivate != nil,
		style: DefaultButtonStyle(), onActivate: onActivate,
	}
}

// Enabled sets whether this action can receive focus and activate
func (a DialogAction[Message]) Enabled(enabled bool) DialogAction[Message] {
	a.enabled = enabled && a.onActivate != nil
	return a
}

// Style replaces this action's button styles
func (a DialogAction[Message]) Style(style ButtonStyle) DialogAction[Message] {
	a.style = style
	return a
}

// ID returns the stable Node ID used by this action
func (a DialogAction[Message]) ID() tui.NodeID {
	return a.id
}

// Label returns the single-line action label
func (a DialogAction[Message]) Label() string {
	return a.label
}

// IsEnabled reports whether this action can receive focus and activate
func (a DialogAction[Message]) IsEnabled() bool {
	return a.enabled
}

func (a DialogAction[Message]) buttonWidth(profile celltext.WidthProfile) uint32 {
	return saturatingAddDialogWidth(cellCount(celltext.Width(a.label, profile)), 4)
}

func (a DialogAction[Message]) node() tui.Node[Message] {
	return NewButton(a.id, a.label, a.onActivate).Enabled(a.enabled).Style(a.style).Node()
}

// Dialog is a centered modal panel with application-defined content and actions
//
// The application explicitly selects default and cancel action IDs. Missing
// selections pass through, while configured IDs that do not name an enabled
// action consume their semantic key so it cannot escape the dialog.
type Dialog[Message any] struct {
	id                 tui.NodeID
	title              *tui.Node[Message]
	body               tui.Node[Message]
	details            *Disclosure[Message]
	actions            []DialogAction[Message]
	defaultAction      *tui.NodeID
	cancelAction       *tui.NodeID
	initialFocus       *tui.ModalInitialFocus
	returnFocus        tui.ModalReturnFocus
	actionWrapWidth    uint32
	hasActionWrapWidth bool
	style              DialogStyle
	widthProfile       celltext.WidthProfile
}

// NewDialog returns an untitled dialog without default or cancel action selection
func NewDialog[Message any](
	id tui.NodeID,
	body tui.Node[Message],
	actions []DialogAction[Message],
) Dialog[Message] {
	ownedActions := append([]DialogAction[Message](nil), actions...)
	return Dialog[Message]{
		id: id, body: body, actions: ownedActions,
		returnFocus: tui.ModalReturnFocusPrevious(), style: DefaultDialogStyle(),
		widthProfile: celltext.ModernWidth(),
	}
}

// Title sets the optional title slot
func (d Dialog[Message]) Title(title tui.Node[Message]) Dialog[Message] {
	d.title = &title
	return d
}

// Details sets the optional controlled and lazy details disclosure
func (d Dialog[Message]) Details(details Disclosure[Message]) Dialog[Message] {
	d.details = &details
	return d
}

// DefaultAction selects the action invoked by the semantic confirmation action
func (d Dialog[Message]) DefaultAction(id tui.NodeID) Dialog[Message] {
	d.defaultAction = &id
	return d
}

// CancelAction selects the action invoked by the semantic dismissal action
func (d Dialog[Message]) CancelAction(id tui.NodeID) Dialog[Message] {
	d.cancelAction = &id
	return d
}

// InitialFocus sets the focus policy used when this dialog becomes active
//
// Without this override, a configured default action is targeted and a dialog
// without one selects the first focusable node.
func (d Dialog[Message]) InitialFocus(focus tui.ModalInitialFocus) Dialog[Message] {
	d.initialFocus = &focus
	return d
}

// ReturnFocus sets the focus policy used when this dialog stops being active
func (d Dialog[Message]) ReturnFocus(focus tui.ModalReturnFocus) Dialog[Message] {
	d.returnFocus = focus
	return d
}

// ActionWrapWidth sets the maximum terminal Cell width used by each action row
//
// Actions keep input order and greedily wrap with one Cell between them. Zero
// is normalized to one, and an oversized action occupies its own row.
func (d Dialog[Message]) ActionWrapWidth(width uint32) Dialog[Message] {
	d.actionWrapWidth = max(width, 1)
	d.hasActionWrapWidth = true
	return d
}

// Style replaces the dialog panel styles
func (d Dialog[Message]) Style(style DialogStyle) Dialog[Message] {
	d.style = style
	return d
}

// WidthProfile sets the terminal cell-width policy used to arrange actions
//
// Pass ViewContext.WidthProfile to keep the widget aligned with its Runtime.
func (d Dialog[Message]) WidthProfile(profile celltext.WidthProfile) Dialog[Message] {
	d.widthProfile = profile
	return d
}

// ActionDescriptors returns confirmation and dismissal descriptors in semantic order
func (d Dialog[Message]) ActionDescriptors() [2]tui.ActionDescriptor {
	return [2]tui.ActionDescriptor{
		ConfirmActionDescriptor().WithAvailability(d.targetAvailability(d.defaultAction)),
		DismissActionDescriptor().WithAvailability(d.targetAvailability(d.cancelAction)),
	}
}

// Node builds the public semantic node for this dialog
func (d Dialog[Message]) Node() tui.Node[Message] {
	descriptors := d.ActionDescriptors()
	onDefault := d.targetHandler(d.defaultAction)
	onCancel := d.targetHandler(d.cancelAction)
	initial := tui.ModalInitialFocusFirst()
	if d.defaultAction != nil {
		initial = tui.ModalInitialFocusTarget(*d.defaultAction)
	}
	if d.initialFocus != nil {
		initial = *d.initialFocus
	}
	focus := tui.ModalFocusOptions{Initial: initial, ReturnFocus: d.returnFocus}

	children := make([]tui.Node[Message], 0, 4)
	if d.title != nil {
		children = append(children, *d.title)
	}
	children = append(children, d.body)
	if d.details != nil {
		children = append(children, d.details.Node())
	}
	if len(d.actions) > 0 {
		children = append(children, dialogActionRows(d.actions, d.actionWrapWidth, d.hasActionWrapWidth, d.widthProfile))
	}
	panel := tui.Border(tui.Column(children...), d.style.Border)
	centered := tui.Align(panel, tui.AlignCenter, tui.AlignMiddle)
	modal := tui.ModalWithFocus(d.id, centered, focus)
	var defaultHandler func(tui.ActionEvent) tui.EventResult[Message]
	if onDefault != nil {
		defaultHandler = func(tui.ActionEvent) tui.EventResult[Message] {
			return tui.MessageResult(onDefault())
		}
	}
	var cancelHandler func(tui.ActionEvent) tui.EventResult[Message]
	if onCancel != nil {
		cancelHandler = func(tui.ActionEvent) tui.EventResult[Message] {
			return tui.MessageResult(onCancel())
		}
	}
	return modal.OnActions(d.id, []tui.Action[Message]{
		tui.NewAction(descriptors[0], defaultHandler),
		tui.NewAction(descriptors[1], cancelHandler),
	})
}

func (d Dialog[Message]) targetAvailability(target *tui.NodeID) tui.ActionAvailability {
	if target == nil {
		return tui.ActionDisabledPassThrough
	}
	for _, action := range d.actions {
		if action.id == *target && action.enabled {
			return tui.ActionEnabled
		}
	}
	return tui.ActionDisabledConsume
}

func (d Dialog[Message]) targetHandler(target *tui.NodeID) func() Message {
	if target == nil {
		return nil
	}
	for _, action := range d.actions {
		if action.id == *target && action.enabled {
			return action.onActivate
		}
	}
	return nil
}

func (d Dialog[Message]) actionStyle(target tui.NodeID, style ButtonStyle) Dialog[Message] {
	for index := range d.actions {
		if d.actions[index].id == target {
			d.actions = append([]DialogAction[Message](nil), d.actions...)
			d.actions[index].style = style
			break
		}
	}
	return d
}

type confirmDialogDefaultKind uint8

const (
	confirmDialogDefaultInvalid confirmDialogDefaultKind = iota
	confirmDialogDefaultConfirm
	confirmDialogDefaultCancel
)

// ConfirmDialogDefault is an explicit default action selected for a ConfirmDialog
//
// Its zero value is invalid so construction cannot silently choose a safety
// policy. Use ConfirmDialogDefaultConfirm or ConfirmDialogDefaultCancel.
type ConfirmDialogDefault struct {
	kind confirmDialogDefaultKind
}

// ConfirmDialogDefaultConfirm makes the confirm action the default
func ConfirmDialogDefaultConfirm() ConfirmDialogDefault {
	return ConfirmDialogDefault{kind: confirmDialogDefaultConfirm}
}

// ConfirmDialogDefaultCancel makes the cancel action the default
func ConfirmDialogDefaultCancel() ConfirmDialogDefault {
	return ConfirmDialogDefault{kind: confirmDialogDefaultCancel}
}

// ConfirmDialog is a two-action convenience over Dialog
//
// The constructor requires an explicit default selection. Three or more
// choices belong in a generic Dialog action list.
type ConfirmDialog[Message any] struct {
	dialog    Dialog[Message]
	confirmID tui.NodeID
}

// NewConfirmDialog returns a confirm-and-cancel dialog with an explicit default action
//
// Passing the zero ConfirmDialogDefault is a programmer error and panics.
func NewConfirmDialog[Message any](
	id tui.NodeID,
	body tui.Node[Message],
	confirm DialogAction[Message],
	cancel DialogAction[Message],
	defaultAction ConfirmDialogDefault,
) ConfirmDialog[Message] {
	var defaultID tui.NodeID
	switch defaultAction.kind {
	case confirmDialogDefaultConfirm:
		defaultID = confirm.id
	case confirmDialogDefaultCancel:
		defaultID = cancel.id
	default:
		panic("widget: ConfirmDialog default action must be explicit")
	}
	return ConfirmDialog[Message]{
		dialog: NewDialog(id, body, []DialogAction[Message]{confirm, cancel}).
			DefaultAction(defaultID).
			CancelAction(cancel.id),
		confirmID: confirm.id,
	}
}

// Title sets the optional title slot
func (d ConfirmDialog[Message]) Title(title tui.Node[Message]) ConfirmDialog[Message] {
	d.dialog = d.dialog.Title(title)
	return d
}

// Details sets the optional controlled and lazy details disclosure
func (d ConfirmDialog[Message]) Details(details Disclosure[Message]) ConfirmDialog[Message] {
	d.dialog = d.dialog.Details(details)
	return d
}

// DestructiveStyle applies an application-supplied destructive style to the confirm action
func (d ConfirmDialog[Message]) DestructiveStyle(style ButtonStyle) ConfirmDialog[Message] {
	d.dialog = d.dialog.actionStyle(d.confirmID, style)
	return d
}

// InitialFocus sets the focus policy used when this dialog becomes active
func (d ConfirmDialog[Message]) InitialFocus(focus tui.ModalInitialFocus) ConfirmDialog[Message] {
	d.dialog = d.dialog.InitialFocus(focus)
	return d
}

// ReturnFocus sets the focus policy used when this dialog stops being active
func (d ConfirmDialog[Message]) ReturnFocus(focus tui.ModalReturnFocus) ConfirmDialog[Message] {
	d.dialog = d.dialog.ReturnFocus(focus)
	return d
}

// ActionWrapWidth sets the maximum terminal Cell width used by each action row
func (d ConfirmDialog[Message]) ActionWrapWidth(width uint32) ConfirmDialog[Message] {
	d.dialog = d.dialog.ActionWrapWidth(width)
	return d
}

// Style replaces the dialog panel styles
func (d ConfirmDialog[Message]) Style(style DialogStyle) ConfirmDialog[Message] {
	d.dialog = d.dialog.Style(style)
	return d
}

// WidthProfile sets the terminal cell-width policy used to arrange actions
//
// Pass ViewContext.WidthProfile to keep the widget aligned with its Runtime.
func (d ConfirmDialog[Message]) WidthProfile(profile celltext.WidthProfile) ConfirmDialog[Message] {
	d.dialog = d.dialog.WidthProfile(profile)
	return d
}

// ActionDescriptors returns confirmation and dismissal descriptors in semantic order
func (d ConfirmDialog[Message]) ActionDescriptors() [2]tui.ActionDescriptor {
	return d.dialog.ActionDescriptors()
}

// Node builds the public semantic node for this confirm dialog
func (d ConfirmDialog[Message]) Node() tui.Node[Message] {
	return d.dialog.Node()
}

func dialogActionRows[Message any](
	actions []DialogAction[Message],
	wrapWidth uint32,
	hasWrapWidth bool,
	profile celltext.WidthProfile,
) tui.Node[Message] {
	rows := make([]tui.Node[Message], 0, len(actions))
	row := make([]tui.Node[Message], 0, len(actions)*2)
	var used uint32
	for _, action := range actions {
		width := action.buttonWidth(profile)
		required := width
		if len(row) > 0 {
			required = saturatingAddDialogWidth(saturatingAddDialogWidth(used, 1), width)
		}
		if len(row) > 0 && hasWrapWidth && required > wrapWidth {
			rows = append(rows, tui.Row(row...))
			row = make([]tui.Node[Message], 0, len(actions)*2)
			used = 0
		}
		if len(row) > 0 {
			row = append(row, tui.Gap[Message](1))
			used = saturatingAddDialogWidth(used, 1)
		}
		row = append(row, action.node())
		used = saturatingAddDialogWidth(used, width)
	}
	if len(row) > 0 {
		rows = append(rows, tui.Row(row...))
	}
	return tui.Column(rows...)
}

func saturatingAddDialogWidth(left, right uint32) uint32 {
	if ^uint32(0)-left < right {
		return ^uint32(0)
	}
	return left + right
}
