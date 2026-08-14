package tui

import celltext "github.com/mayahiro/nagi-go/text"

// ViewContext contains environment information available while rebuilding a
// semantic view
type ViewContext struct {
	// Size is the current terminal size in cells
	Size Size
	// WidthProfile is the runtime terminal cell-width policy
	WidthProfile celltext.WidthProfile
	// TerminalCapabilities contains detected features and active input mode
	TerminalCapabilities TerminalCapabilityProfile
}

// App is one application whose state is updated by sequential messages
type App[Message any] interface {
	// Init initializes application state and returns startup work
	Init() Effect[Message]
	// Update applies one message and returns follow-up work
	Update(Message) Effect[Message]
	// Subscriptions describes long-lived message sources for the current state
	Subscriptions() Subscription[Message]
	// View rebuilds the semantic view for the current state
	View(ViewContext) Node[Message]
}
