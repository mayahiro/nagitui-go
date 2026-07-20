package tui

import (
	"errors"
	"fmt"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go/surface"
)

// DefaultQueueCapacity is the default maximum number of waiting messages
const DefaultQueueCapacity = 4_096

// DefaultTaskLimit is the default maximum number of concurrent effect tasks
const DefaultTaskLimit = 64

// DefaultSubscriptionCapacity is the default per-source subscription inbox
// capacity
const DefaultSubscriptionCapacity = 256

var (
	// ErrZeroQueueCapacity indicates that runtime queue capacity is zero
	ErrZeroQueueCapacity = errors.New("runtime queue capacity must be positive")
	// ErrQueueFull indicates that the bounded runtime message queue is full
	ErrQueueFull = errors.New("runtime message queue is full")
	// ErrZeroTaskLimit indicates that concurrent task limit is zero
	ErrZeroTaskLimit = errors.New("runtime task limit must be positive")
	// ErrZeroSubscriptionCapacity indicates that subscription capacity is zero
	ErrZeroSubscriptionCapacity = errors.New("runtime subscription capacity must be positive")
	// ErrNegativeFrameInterval indicates that a frame interval is negative
	ErrNegativeFrameInterval = errors.New("runtime minimum frame interval must not be negative")
)

// RuntimeConfig contains runtime construction settings
type RuntimeConfig struct {
	// Size is the initial terminal cell size
	Size Size
	// QueueCapacity is the maximum number of waiting messages
	QueueCapacity int
	// TaskLimit is the maximum number of effect tasks executing concurrently
	TaskLimit int
	// SubscriptionCapacity is the maximum pending values retained per source
	SubscriptionCapacity int
	// MinimumFrameInterval limits non-urgent rendering; zero disables the limit
	MinimumFrameInterval time.Duration
}

// NewRuntimeConfig returns settings with the default bounded queue capacity
func NewRuntimeConfig(size Size) RuntimeConfig {
	return RuntimeConfig{
		Size:                 size,
		QueueCapacity:        DefaultQueueCapacity,
		TaskLimit:            DefaultTaskLimit,
		SubscriptionCapacity: DefaultSubscriptionCapacity,
	}
}

// Frame is one coalesced rendered frame
type Frame struct {
	timestamp  Timestamp
	surface    *surface.Surface
	operations []vt.TerminalOp
}

// Timestamp returns the monotonic time at which the frame was produced
func (f Frame) Timestamp() Timestamp {
	return f.timestamp
}

// Surface returns an independent copy of the normalized rendered surface
func (f Frame) Surface() *surface.Surface {
	return f.surface.Clone()
}

// Operations returns a copy of typed VT operations relative to the previous
// frame
func (f Frame) Operations() []vt.TerminalOp {
	return append([]vt.TerminalOp(nil), f.operations...)
}

// Runtime is a single-goroutine application runtime with a bounded FIFO queue
type Runtime[Message any] struct {
	app                  App[Message]
	clock                Clock
	size                 Size
	queue                []queuedMessage[Message]
	queueCapacity        int
	dirty                bool
	urgentFrame          bool
	minimumFrameInterval time.Duration
	lastFrame            Timestamp
	hasLastFrame         bool
	previousSurface      *surface.Surface
	interaction          *InteractionState
	viewTree             *Node[Message]
	treeIndex            treeIndex
	nextTreeIndex        treeIndex
	effects              *effectSupervisor[Message]
	subscriptions        *subscriptionSupervisor[Message]
	subscriptionsDirty   bool
	exitRequested        bool
	pendingFocus         NodeID
	hasPendingFocus      bool
	pendingScroll        []pendingScrollRequest
}

type pendingScrollRequest struct {
	id     NodeID
	offset ScrollOffset
}

type queuedMessage[Message any] struct {
	message      Message
	subscription *subscriptionTag
}

// NewRuntime returns a runtime using a production monotonic clock
func NewRuntime[Message any](app App[Message], size Size) (*Runtime[Message], error) {
	return NewRuntimeWithClock(app, NewRuntimeConfig(size), NewSystemClock())
}

// NewRuntimeWithClock returns a runtime using explicit settings and clock
func NewRuntimeWithClock[Message any](app App[Message], config RuntimeConfig, clock Clock) (*Runtime[Message], error) {
	return newRuntimeWithClockAndWake(app, config, clock, nil)
}

func newRuntimeWithClockAndWake[Message any](
	app App[Message],
	config RuntimeConfig,
	clock Clock,
	wake runtimeWake,
) (*Runtime[Message], error) {
	if config.QueueCapacity <= 0 {
		return nil, ErrZeroQueueCapacity
	}
	if config.TaskLimit <= 0 {
		return nil, ErrZeroTaskLimit
	}
	if config.SubscriptionCapacity <= 0 {
		return nil, ErrZeroSubscriptionCapacity
	}
	if config.MinimumFrameInterval < 0 {
		return nil, ErrNegativeFrameInterval
	}
	if app == nil {
		return nil, errors.New("nagi-tui: nil application")
	}
	if clock == nil {
		return nil, errors.New("nagi-tui: nil clock")
	}
	startup := app.Init()
	declaredSubscriptions := app.Subscriptions()
	effects := newEffectSupervisor[Message](config.TaskLimit)
	effects.wake = wake
	effects.schedule(startup, clock.Now())
	subscriptions := newSubscriptionSupervisor[Message](config.SubscriptionCapacity)
	subscriptions.wake = wake
	if _, err := subscriptions.reconcile(declaredSubscriptions, clock.Now()); err != nil {
		effects.close()
		return nil, err
	}
	runtime := &Runtime[Message]{
		app:                  app,
		clock:                clock,
		size:                 config.Size,
		queue:                make([]queuedMessage[Message], 0, min(config.QueueCapacity, 64)),
		queueCapacity:        config.QueueCapacity,
		dirty:                true,
		urgentFrame:          true,
		minimumFrameInterval: config.MinimumFrameInterval,
		interaction:          NewInteractionState(),
		treeIndex:            newTreeIndex(),
		nextTreeIndex:        newTreeIndex(),
		effects:              effects,
		subscriptions:        subscriptions,
	}
	runtime.applyEffectCommands()
	return runtime, nil
}

// Close cooperatively cancels active effect tasks and timers
func (r *Runtime[Message]) Close() {
	r.effects.close()
	r.subscriptions.close()
}

// App returns the application instance owned by the runtime
func (r *Runtime[Message]) App() App[Message] {
	return r.app
}

// Size returns the current terminal cell size
func (r *Runtime[Message]) Size() Size {
	return r.size
}

// Interaction returns runtime-owned UI continuity
func (r *Runtime[Message]) Interaction() *InteractionState {
	return r.interaction
}

// Resize changes terminal size and schedules a frame when it differs
func (r *Runtime[Message]) Resize(size Size) {
	if r.size != size {
		r.size = size
		r.dirty = true
		r.urgentFrame = true
	}
}

// Enqueue adds one message to the FIFO queue
func (r *Runtime[Message]) Enqueue(message Message) error {
	if len(r.queue) >= r.queueCapacity {
		return ErrQueueFull
	}
	r.queue = append(r.queue, queuedMessage[Message]{message: message})
	return nil
}

// QueuedMessages returns the number of waiting messages
func (r *Runtime[Message]) QueuedMessages() int {
	return len(r.queue)
}

// PollEffects moves due timers and completed tasks into the bounded queue
func (r *Runtime[Message]) PollEffects() int {
	r.effects.poll(r.clock.Now())
	r.applyEffectCommands()
	available := r.queueCapacity - len(r.queue)
	messages := r.effects.takeReady(available)
	for _, message := range messages {
		r.queue = append(r.queue, queuedMessage[Message]{message: message})
	}
	return len(messages)
}

// ExitRequested reports whether the application requested normal exit
func (r *Runtime[Message]) ExitRequested() bool {
	return r.exitRequested
}

// PollSubscriptions moves ready subscription values into the bounded queue
func (r *Runtime[Message]) PollSubscriptions() int {
	r.subscriptions.poll(r.clock.Now())
	available := r.queueCapacity - len(r.queue)
	messages := r.subscriptions.takeReady(available)
	for _, delivery := range messages {
		tag := delivery.tag
		r.queue = append(r.queue, queuedMessage[Message]{message: delivery.message, subscription: &tag})
	}
	return len(messages)
}

// ActiveTasks returns supervised tasks that have not fully finished
func (r *Runtime[Message]) ActiveTasks() int {
	return r.effects.activeTasks()
}

// RunningTasks returns supervised tasks occupying worker slots
func (r *Runtime[Message]) RunningTasks() int {
	return r.effects.runningTasks()
}

// PendingTasks returns supervised tasks waiting for a worker slot
func (r *Runtime[Message]) PendingTasks() int {
	return r.effects.pendingTasks()
}

// PendingEffectMessages returns completed effect messages waiting for capacity
func (r *Runtime[Message]) PendingEffectMessages() int {
	return len(r.effects.ready)
}

// EffectDiagnostics returns cancellation, stale-result, and panic counters
func (r *Runtime[Message]) EffectDiagnostics() EffectDiagnostics {
	return r.effects.diagnostics
}

// TaskGeneration returns the latest generation started for one task key
func (r *Runtime[Message]) TaskGeneration(key TaskKey) uint64 {
	return r.effects.generation(key)
}

// ActiveSubscriptions returns the number of currently declared sources
func (r *Runtime[Message]) ActiveSubscriptions() int {
	return r.subscriptions.activeSubscriptions()
}

// RunningSubscriptionStreams returns Stream producers that have not returned
func (r *Runtime[Message]) RunningSubscriptionStreams() int {
	return r.subscriptions.runningStreams()
}

// PendingSubscriptionMessages returns values waiting before the application
// queue
func (r *Runtime[Message]) PendingSubscriptionMessages() int {
	return r.subscriptions.pendingMessages()
}

// SubscriptionActive reports whether key is currently declared
func (r *Runtime[Message]) SubscriptionActive(key SubscriptionKey) bool {
	return r.subscriptions.isActive(key)
}

// SubscriptionGeneration returns the latest started generation for key
func (r *Runtime[Message]) SubscriptionGeneration(key SubscriptionKey) uint64 {
	return r.subscriptions.generation(key)
}

// SubscriptionDiagnostics returns lifecycle and backpressure counters
func (r *Runtime[Message]) SubscriptionDiagnostics() SubscriptionDiagnostics {
	return r.subscriptions.diagnostics()
}

// TimeUntilEffectDeadline returns time until the next clock-driven deadline
func (r *Runtime[Message]) TimeUntilEffectDeadline() (time.Duration, bool) {
	return r.effects.timeUntilDeadline(r.clock.Now())
}

// TimeUntilSubscriptionDeadline returns time until the next subscription
// deadline
func (r *Runtime[Message]) TimeUntilSubscriptionDeadline() (time.Duration, bool) {
	return r.subscriptions.timeUntilDeadline(r.clock.Now())
}

// TimeUntilFrameDeadline returns time until a pending rate-limited frame
func (r *Runtime[Message]) TimeUntilFrameDeadline() (time.Duration, bool) {
	if !r.dirty || r.urgentFrame || r.minimumFrameInterval == 0 || !r.hasLastFrame {
		return 0, false
	}
	deadline := r.lastFrame.Add(r.minimumFrameInterval)
	if deadline <= r.clock.Now() {
		return 0, true
	}
	return time.Duration(deadline - r.clock.Now()), true
}

// ProcessPending applies every queued message in FIFO order without rendering
// between messages
func (r *Runtime[Message]) ProcessPending() (int, error) {
	return r.ProcessPendingWith(nil)
}

// ProcessPendingWith applies queued messages and observes each immediately
// before Update
func (r *Runtime[Message]) ProcessPendingWith(observe func(Message)) (int, error) {
	if err := r.reconcileSubscriptions(); err != nil {
		return 0, err
	}
	r.PollEffects()
	r.PollSubscriptions()
	processed := 0
	for len(r.queue) > 0 {
		queued := r.queue[0]
		var zero queuedMessage[Message]
		r.queue[0] = zero
		r.queue = r.queue[1:]
		if observe != nil {
			observe(queued.message)
		}
		effect := r.app.Update(queued.message)
		r.effects.schedule(effect, r.clock.Now())
		r.applyEffectCommands()
		r.dirty = true
		r.subscriptionsDirty = true
		if err := r.reconcileSubscriptions(); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (r *Runtime[Message]) applyEffectCommands() {
	for _, command := range r.effects.takeCommands() {
		switch command.kind {
		case runtimeCommandExit:
			r.exitRequested = true
		case runtimeCommandFocus:
			r.pendingFocus = command.id
			r.hasPendingFocus = true
		case runtimeCommandScrollTo:
			r.pendingScroll = append(r.pendingScroll, pendingScrollRequest{
				id:     command.id,
				offset: command.offset,
			})
		default:
			panic("nagi-tui: invalid runtime command")
		}
		r.dirty = true
		r.urgentFrame = true
	}
}

func (r *Runtime[Message]) applyPendingInteraction(index treeIndex) {
	if r.hasPendingFocus {
		if index.allowsFocus(r.pendingFocus) {
			r.interaction.focused = r.pendingFocus
			r.interaction.hasFocus = true
		}
		r.pendingFocus = ""
		r.hasPendingFocus = false
	}
	for _, request := range r.pendingScroll {
		record, ok := index.record(request.id)
		if ok && record.kind == interactiveScrollViewport {
			r.interaction.requestScroll(request.id, request.offset)
		}
	}
	r.pendingScroll = nil
}

// RequestFrame schedules a frame even when application state has not changed
func (r *Runtime[Message]) RequestFrame() {
	r.dirty = true
	r.urgentFrame = true
	r.subscriptionsDirty = true
}

// RequestFocus requests focus for an ID in the current semantic tree
func (r *Runtime[Message]) RequestFocus(id NodeID) (bool, error) {
	if err := r.ensureTree(); err != nil {
		return false, err
	}
	if !r.treeIndex.allowsFocus(id) {
		return false, nil
	}
	if !r.interaction.hasFocus || r.interaction.focused != id {
		r.interaction.focused = id
		r.interaction.hasFocus = true
		r.dirty = true
		r.urgentFrame = true
	}
	return true, nil
}

func (r *Runtime[Message]) focusFirst() (bool, error) {
	if err := r.ensureTree(); err != nil {
		return false, err
	}
	focusScope := r.treeIndex.focusScope()
	if len(focusScope) == 0 {
		return false, nil
	}
	return r.RequestFocus(focusScope[0])
}

// ClearFocus releases node focus and schedules a frame when focus existed
func (r *Runtime[Message]) ClearFocus() {
	if r.interaction.hasFocus {
		r.interaction.focused = ""
		r.interaction.hasFocus = false
		r.dirty = true
		r.urgentFrame = true
	}
}

// SetTextCursor sets a UTF-8 byte cursor normalized to a grapheme boundary
func (r *Runtime[Message]) SetTextCursor(id NodeID, cursor int) bool {
	state, ok := r.interaction.textInputs[id]
	if !ok {
		return false
	}
	normalized := normalizeTextCursor(state.draft, cursor)
	if state.cursor != normalized {
		state.cursor = normalized
		r.dirty = true
		r.urgentFrame = true
	}
	return true
}

// SetScrollOffset requests an offset that is clamped during layout
func (r *Runtime[Message]) SetScrollOffset(id NodeID, offset ScrollOffset) bool {
	_, ok := r.interaction.ScrollState(id)
	if !ok {
		return false
	}
	_, changed, _ := r.interaction.requestScroll(id, offset)
	if changed {
		r.dirty = true
		r.urgentFrame = true
	}
	return true
}

// DispatchEvent routes one normalized event through focus, hit testing, and
// ancestors
func (r *Runtime[Message]) DispatchEvent(event vt.Event) (EventDispatch, error) {
	if err := r.ensureTree(); err != nil {
		return EventDispatch{}, err
	}
	if event.Kind == vt.EventKey && event.Key.Action != vt.KeyRelease &&
		event.Key.Code == vt.KeyTab && !event.Key.Modifiers.Alt &&
		!event.Key.Modifiers.Control && !event.Key.Modifiers.Meta {
		focused := optionalNodeID(r.interaction.focused, r.interaction.hasFocus)
		r.interaction.focused, r.interaction.hasFocus = traverseFocus(
			r.treeIndex.focusScope(),
			focused,
			!event.Key.Modifiers.Shift,
		)
		r.dirty = true
		r.urgentFrame = true
		return EventDispatch{consumed: true, redraw: true}, nil
	}

	var target NodeID
	hasTarget := false
	if event.Kind == vt.EventMouse {
		if r.interaction.hasCapture {
			target, hasTarget = r.interaction.pointerCapture, true
		} else {
			x := int32(min(event.Mouse.X, uint32(1<<31-1)))
			y := int32(min(event.Mouse.Y, uint32(1<<31-1)))
			target, hasTarget = r.treeIndex.hitTest(Point{X: x, Y: y})
		}
	} else if r.interaction.hasFocus {
		target, hasTarget = r.interaction.focused, true
	}
	if event.Kind == vt.EventMouse && event.Mouse.Kind == vt.MousePress && hasTarget {
		if r.treeIndex.allowsFocus(target) {
			r.interaction.focused = target
			r.interaction.hasFocus = true
			r.dirty = true
			r.urgentFrame = true
		}
	}

	dispatch := EventDispatch{}
	for index, id := range r.treeIndex.route(target, hasTarget) {
		record, _ := r.treeIndex.record(id)
		var result EventResult[Message]
		var handled bool
		switch {
		case record.kind == interactiveTextInput && index == 0:
			result, handled = r.handleTextInput(id, event)
		case record.kind == interactiveScrollViewport:
			result, handled = r.handleScroll(id, event)
		}
		if handled {
			if err := r.applyEventResult(result, &dispatch); err != nil {
				return dispatch, err
			}
			if dispatch.consumed {
				break
			}
		}
		if result, ok := r.viewTree.handleEvent(id, event); ok {
			if err := r.applyEventResult(result, &dispatch); err != nil {
				return dispatch, err
			}
			if dispatch.consumed {
				break
			}
		}
	}
	return dispatch, nil
}

func (r *Runtime[Message]) handleTextInput(id NodeID, event vt.Event) (EventResult[Message], bool) {
	kind := textEditInsert
	inserted := ""
	switch event.Kind {
	case vt.EventText:
		inserted = event.Text
	case vt.EventPaste:
		kind = textEditPaste
		inserted = event.Text
	case vt.EventKey:
		if event.Key.Action == vt.KeyRelease {
			return EventResult[Message]{}, false
		}
		switch event.Key.Code {
		case vt.KeyLeft:
			kind = textEditLeft
		case vt.KeyRight:
			kind = textEditRight
		case vt.KeyHome:
			kind = textEditHome
		case vt.KeyEnd:
			kind = textEditEnd
		case vt.KeyBackspace:
			kind = textEditBackspace
		case vt.KeyDelete:
			kind = textEditDelete
		case vt.KeyCharacter:
			if event.Key.Modifiers.Control || event.Key.Modifiers.Meta || !event.Key.HasText {
				return EventResult[Message]{}, false
			}
			inserted = event.Key.Text
		default:
			return EventResult[Message]{}, false
		}
	default:
		return EventResult[Message]{}, false
	}
	state, ok := r.interaction.textInputs[id]
	if !ok {
		return EventResult[Message]{}, false
	}
	previous := state.draft
	value, cursor := applyTextEdit(previous, state.cursor, kind, inserted)
	state.draft = value
	state.cursor = cursor
	r.dirty = true
	r.urgentFrame = true
	result := ConsumeResult[Message]().Redraw()
	if value != previous {
		message, ok := r.viewTree.textInputMessage(id, value)
		if !ok {
			return EventResult[Message]{}, false
		}
		result = result.Emit(message)
	}
	return result, true
}

func (r *Runtime[Message]) handleScroll(id NodeID, event vt.Event) (EventResult[Message], bool) {
	state, ok := r.interaction.ScrollState(id)
	if !ok {
		return EventResult[Message]{}, false
	}
	options, ok := r.viewTree.scrollOptions(id)
	if !ok {
		return EventResult[Message]{}, false
	}
	viewport, ok := r.treeIndex.record(id)
	if !ok {
		return EventResult[Message]{}, false
	}
	next := state.Offset
	handled := true
	switch {
	case event.Kind == vt.EventMouse && event.Mouse.Kind == vt.MouseScroll:
		switch event.Mouse.Button {
		case vt.MouseWheelUp:
			handled = options.Axis.allowsVertical()
			if handled {
				next.Y -= min(next.Y, 3)
			}
		case vt.MouseWheelDown:
			handled = options.Axis.allowsVertical()
			if handled {
				next.Y = saturatingAdd32(next.Y, 3)
			}
		case vt.MouseWheelLeft:
			handled = options.Axis.allowsHorizontal()
			if handled {
				next.X -= min(next.X, 3)
			}
		case vt.MouseWheelRight:
			handled = options.Axis.allowsHorizontal()
			if handled {
				next.X = saturatingAdd32(next.X, 3)
			}
		default:
			handled = false
		}
	case event.Kind == vt.EventKey && event.Key.Action != vt.KeyRelease:
		switch event.Key.Code {
		case vt.KeyPageUp:
			handled = options.Axis.allowsVertical()
			if handled {
				next.Y -= min(next.Y, max(viewport.rect.Height, uint32(1)))
			}
		case vt.KeyPageDown:
			handled = options.Axis.allowsVertical()
			if handled {
				next.Y = saturatingAdd32(next.Y, max(viewport.rect.Height, uint32(1)))
			}
		case vt.KeyHome:
			if options.Axis.allowsVertical() {
				next.Y = 0
			} else if options.Axis.allowsHorizontal() {
				next.X = 0
			} else {
				handled = false
			}
		case vt.KeyEnd:
			if options.Axis.allowsVertical() {
				next.Y = state.Maximum.Y
			} else if options.Axis.allowsHorizontal() {
				next.X = state.Maximum.X
			} else {
				handled = false
			}
		default:
			handled = false
		}
	default:
		handled = false
	}
	if !handled {
		return EventResult[Message]{}, false
	}
	state, changed, _ := r.interaction.requestScroll(id, next)
	r.dirty = true
	r.urgentFrame = true
	result := ConsumeResult[Message]().Redraw()
	if changed {
		if message, ok := r.viewTree.scrollMessage(id, state); ok {
			result = result.Emit(message)
		}
	}
	return result, true
}

func (r *Runtime[Message]) applyEventResult(result EventResult[Message], dispatch *EventDispatch) error {
	switch result.focusChange {
	case focusSet:
		if r.treeIndex.allowsFocus(result.focusID) {
			r.interaction.focused = result.focusID
			r.interaction.hasFocus = true
			r.dirty = true
			r.urgentFrame = true
		}
	case focusRelease:
		r.interaction.focused = ""
		r.interaction.hasFocus = false
		r.dirty = true
		r.urgentFrame = true
	}
	switch result.pointerChange {
	case pointerCapture:
		if r.treeIndex.allowsInteraction(result.pointerID) {
			r.interaction.pointerCapture = result.pointerID
			r.interaction.hasCapture = true
		}
	case pointerRelease:
		r.interaction.pointerCapture = ""
		r.interaction.hasCapture = false
	}
	for _, message := range result.messages {
		if err := r.Enqueue(message); err != nil {
			return err
		}
		dispatch.messages++
	}
	dispatch.consumed = dispatch.consumed || result.consumed
	dispatch.redraw = dispatch.redraw || result.redraw
	if result.redraw {
		r.dirty = true
		r.urgentFrame = true
	}
	return nil
}

func (r *Runtime[Message]) reconcileSubscriptions() error {
	if !r.subscriptionsDirty {
		return nil
	}
	reconciliation, err := r.subscriptions.reconcile(r.app.Subscriptions(), r.clock.Now())
	if err != nil {
		return err
	}
	r.subscriptionsDirty = false
	if len(reconciliation.stopped) == 0 {
		return nil
	}
	stopped := make(map[subscriptionTag]struct{}, len(reconciliation.stopped))
	for _, tag := range reconciliation.stopped {
		stopped[tag] = struct{}{}
	}
	retained := r.queue[:0]
	discarded := 0
	for _, queued := range r.queue {
		if queued.subscription != nil {
			if _, remove := stopped[*queued.subscription]; remove {
				discarded++
				continue
			}
		}
		retained = append(retained, queued)
	}
	for index := len(retained); index < len(r.queue); index++ {
		var zero queuedMessage[Message]
		r.queue[index] = zero
	}
	r.queue = retained
	r.subscriptions.noteDiscarded(discarded)
	return nil
}

func (r *Runtime[Message]) ensureFocusedVisible(view *Node[Message], index *treeIndex) error {
	if !r.interaction.hasFocus {
		return nil
	}
	focused := r.interaction.focused
	route := index.route(focused, true)
	var scrolls []NodeID
	for _, id := range route {
		options, ok := view.scrollOptions(id)
		if ok && options.EnsureFocusedVisible {
			scrolls = append(scrolls, id)
		}
	}
	for _, id := range scrolls {
		target, ok := index.record(focused)
		if !ok {
			break
		}
		viewport, ok := index.record(id)
		if !ok {
			continue
		}
		options, ok := view.scrollOptions(id)
		if !ok {
			continue
		}
		state, ok := r.interaction.ScrollState(id)
		if !ok {
			continue
		}
		next := state.Offset
		if options.Axis.allowsHorizontal() {
			next.X = visibleAxisOffset(next.X, viewport.rect.X, viewport.rect.Width, target.rect.X, target.rect.Width)
		}
		if options.Axis.allowsVertical() {
			next.Y = visibleAxisOffset(next.Y, viewport.rect.Y, viewport.rect.Height, target.rect.Y, target.rect.Height)
		}
		if next == state.Offset {
			continue
		}
		r.interaction.requestScroll(id, next)
		view.prepareInteraction(r.size, r.interaction)
		if err := view.buildTreeIndex(r.size, r.interaction, index); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime[Message]) ensureTree() error {
	if r.viewTree != nil {
		return nil
	}
	view := r.app.View(ViewContext{Size: r.size})
	index := &r.nextTreeIndex
	if err := view.buildTreeIndex(r.size, r.interaction, index); err != nil {
		return err
	}
	r.interaction.reconcile(index.active, nil, index.focusScope())
	if r.interaction.hasCapture && !index.allowsInteraction(r.interaction.pointerCapture) {
		r.interaction.pointerCapture = ""
		r.interaction.hasCapture = false
	}
	r.applyPendingInteraction(*index)
	view.prepareInteraction(r.size, r.interaction)
	if err := view.buildTreeIndex(r.size, r.interaction, index); err != nil {
		return err
	}
	if err := r.ensureFocusedVisible(&view, index); err != nil {
		return err
	}
	r.viewTree = &view
	r.treeIndex, r.nextTreeIndex = r.nextTreeIndex, r.treeIndex
	return nil
}

// RenderIfDirty renders one frame when requested or state changed
func (r *Runtime[Message]) RenderIfDirty() (*Frame, error) {
	if err := r.reconcileSubscriptions(); err != nil {
		return nil, err
	}
	if !r.dirty {
		return nil, nil
	}
	now := r.clock.Now()
	if !r.urgentFrame && r.minimumFrameInterval > 0 && r.hasLastFrame && now < r.lastFrame.Add(r.minimumFrameInterval) {
		return nil, nil
	}
	view := r.app.View(ViewContext{Size: r.size})
	index := &r.nextTreeIndex
	if err := view.buildTreeIndex(r.size, r.interaction, index); err != nil {
		return nil, err
	}
	r.interaction.reconcile(index.active, r.treeIndex.focusScope(), index.focusScope())
	if r.interaction.hasCapture && !index.allowsInteraction(r.interaction.pointerCapture) {
		r.interaction.pointerCapture = ""
		r.interaction.hasCapture = false
	}
	r.applyPendingInteraction(*index)
	view.prepareInteraction(r.size, r.interaction)
	if err := view.buildTreeIndex(r.size, r.interaction, index); err != nil {
		return nil, err
	}
	if err := r.ensureFocusedVisible(&view, index); err != nil {
		return nil, err
	}
	current, err := surface.New(r.size.Width, r.size.Height)
	if err != nil {
		return nil, fmt.Errorf("construct runtime surface: %w", err)
	}
	view.renderTo(current, r.interaction)
	operations := rendererOperations(r.previousSurface, current)
	// Frame only exposes independent clones, so the immutable rendered surface
	// can also serve as the next diff baseline without duplicating its cells.
	r.previousSurface = current
	r.viewTree = &view
	r.treeIndex, r.nextTreeIndex = r.nextTreeIndex, r.treeIndex
	r.dirty = false
	r.urgentFrame = false
	r.lastFrame = now
	r.hasLastFrame = true
	return &Frame{
		timestamp:  now,
		surface:    current,
		operations: operations,
	}, nil
}

func visibleAxisOffset(current uint32, viewportStart int32, viewportSize uint32, targetStart int32, targetSize uint32) uint32 {
	if viewportSize == 0 || targetSize == 0 {
		return current
	}
	viewportBegin := int64(viewportStart)
	viewportEnd := viewportBegin + int64(viewportSize)
	targetBegin := int64(targetStart)
	targetEnd := targetBegin + int64(targetSize)
	switch {
	case targetBegin < viewportBegin:
		delta := uint32(min(viewportBegin-targetBegin, int64(^uint32(0))))
		return current - min(current, delta)
	case targetEnd > viewportEnd:
		delta := uint32(min(targetEnd-viewportEnd, int64(^uint32(0))))
		return saturatingAdd32(current, delta)
	default:
		return current
	}
}

// Step processes all queued messages and produces at most one frame
func (r *Runtime[Message]) Step() (*Frame, error) {
	if _, err := r.ProcessPending(); err != nil {
		return nil, err
	}
	return r.RenderIfDirty()
}
