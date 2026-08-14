package tui

import (
	"context"
	"sort"
	"time"
)

// EffectDiagnostics contains counters for supervised task behavior
type EffectDiagnostics struct {
	cancellations uint64
	staleResults  uint64
	taskPanics    uint64
}

// Cancellations returns the number of cooperative cancellation requests
func (d EffectDiagnostics) Cancellations() uint64 {
	return d.cancellations
}

// StaleResults returns the number of completed task results suppressed as stale
func (d EffectDiagnostics) StaleResults() uint64 {
	return d.staleResults
}

// TaskPanics returns the number of panics caught at the task boundary
func (d EffectDiagnostics) TaskPanics() uint64 {
	return d.taskPanics
}

type scopeTag struct {
	id         ScopeID
	generation uint64
}

type continuationKind uint8

const (
	continuationNone continuationKind = iota
	continuationSequence
	continuationBarrier
)

type effectContinuation struct {
	kind continuationKind
	id   uint64
}

type effectTaskStatus uint8

const (
	effectTaskPending effectTaskStatus = iota
	effectTaskRunning
)

type effectTaskState[Message any] struct {
	task            Task[Message]
	status          effectTaskStatus
	context         context.Context
	cancel          context.CancelFunc
	scopes          []scopeTag
	continuation    effectContinuation
	hasContinuation bool
	latestKey       TaskKey
	latest          latestTask
	hasLatest       bool
	cancelled       bool
}

type terminalTaskState[Message any] struct {
	task            Task[Message]
	status          effectTaskStatus
	context         context.Context
	cancel          context.CancelFunc
	scopes          []scopeTag
	continuation    effectContinuation
	hasContinuation bool
	cancelled       bool
}

type latestTask struct {
	id         uint64
	generation uint64
}

type effectTaskOutcome[Message any] struct {
	id       uint64
	message  Message
	panicked bool
}

type effectTimer[Message any] struct {
	deadline     Timestamp
	order        uint64
	message      Message
	scopes       []scopeTag
	continuation effectContinuation
}

type effectSequenceState[Message any] struct {
	remaining []Effect[Message]
	scopes    []scopeTag
	parent    effectContinuation
}

type effectBarrier struct {
	remaining int
	parent    effectContinuation
}

type runtimeCommandKind uint8

const (
	runtimeCommandExit runtimeCommandKind = iota
	runtimeCommandFocus
	runtimeCommandScrollTo
	runtimeCommandSetClipboard
)

type runtimeCommand struct {
	kind      runtimeCommandKind
	id        NodeID
	offset    ScrollOffset
	clipboard ClipboardRequest
}

type effectSupervisor[Message any] struct {
	parent           context.Context
	taskLimit        int
	wake             runtimeWake
	outcomes         chan effectTaskOutcome[Message]
	tasks            map[uint64]*effectTaskState[Message]
	pending          []uint64
	running          int
	terminalTasks    map[uint64]*terminalTaskState[Message]
	pendingTerminal  []uint64
	latest           map[TaskKey]latestTask
	generations      map[TaskKey]uint64
	scopeGenerations map[ScopeID]uint64
	timers           []effectTimer[Message]
	sequences        map[uint64]*effectSequenceState[Message]
	barriers         map[uint64]*effectBarrier
	ready            []Message
	commands         []runtimeCommand
	notices          *runtimeNoticeQueue
	nextIdentifier   uint64
	nextOrder        uint64
	diagnostics      EffectDiagnostics
	closed           bool
	workers          *workerTracker
}

func newEffectSupervisor[Message any](taskLimit int) *effectSupervisor[Message] {
	return newEffectSupervisorContext[Message](context.Background(), taskLimit)
}

func newEffectSupervisorContext[Message any](parent context.Context, taskLimit int) *effectSupervisor[Message] {
	return &effectSupervisor[Message]{
		parent:           parent,
		taskLimit:        taskLimit,
		outcomes:         make(chan effectTaskOutcome[Message], taskLimit),
		tasks:            make(map[uint64]*effectTaskState[Message]),
		terminalTasks:    make(map[uint64]*terminalTaskState[Message]),
		latest:           make(map[TaskKey]latestTask),
		generations:      make(map[TaskKey]uint64),
		scopeGenerations: make(map[ScopeID]uint64),
		sequences:        make(map[uint64]*effectSequenceState[Message]),
		barriers:         make(map[uint64]*effectBarrier),
		nextIdentifier:   1,
		workers:          &workerTracker{},
	}
}

func (s *effectSupervisor[Message]) schedule(effect Effect[Message], now Timestamp) {
	if s.closed {
		return
	}
	s.startEffect(effect, nil, effectContinuation{}, now)
}

func (s *effectSupervisor[Message]) poll(now Timestamp) {
	if s.closed {
		return
	}
	s.pollTimers(now)
	for {
		select {
		case outcome := <-s.outcomes:
			s.finishTask(outcome, now)
		default:
			s.spawnAvailable(now)
			return
		}
	}
}

func (s *effectSupervisor[Message]) takeReady(maximum int) []Message {
	count := min(max(maximum, 0), len(s.ready))
	messages := append([]Message(nil), s.ready[:count]...)
	var zero Message
	for index := range s.ready[:count] {
		s.ready[index] = zero
	}
	s.ready = s.ready[count:]
	return messages
}

func (s *effectSupervisor[Message]) popReady() (Message, bool) {
	if len(s.ready) == 0 {
		var zero Message
		return zero, false
	}
	message := s.ready[0]
	var zero Message
	s.ready[0] = zero
	if len(s.ready) == 1 {
		s.ready = s.ready[:0]
	} else {
		s.ready = s.ready[1:]
	}
	return message, true
}

func (s *effectSupervisor[Message]) takeCommands() []runtimeCommand {
	commands := append([]runtimeCommand(nil), s.commands...)
	s.commands = nil
	return commands
}

func (s *effectSupervisor[Message]) activeTasks() int {
	return len(s.tasks)
}

func (s *effectSupervisor[Message]) runningTasks() int {
	return s.running
}

func (s *effectSupervisor[Message]) pendingTasks() int {
	count := 0
	for _, task := range s.tasks {
		if task.status == effectTaskPending {
			count++
		}
	}
	return count
}

func (s *effectSupervisor[Message]) pendingTerminalTasks() int {
	count := 0
	for _, task := range s.terminalTasks {
		if task.status == effectTaskPending {
			count++
		}
	}
	return count
}

func (s *effectSupervisor[Message]) runTerminalTask(now Timestamp) bool {
	for len(s.pendingTerminal) > 0 {
		id := s.pendingTerminal[0]
		s.pendingTerminal = s.pendingTerminal[1:]
		state := s.terminalTasks[id]
		if state == nil || state.cancelled || state.status != effectTaskPending {
			continue
		}
		state.status = effectTaskRunning
		task := state.task
		state.task = nil
		message, panicked := runEffectTask(task, state.context)
		s.finishTerminalTask(id, message, panicked, now)
		return true
	}
	return false
}

func (s *effectSupervisor[Message]) generation(key TaskKey) uint64 {
	return s.generations[key]
}

func (s *effectSupervisor[Message]) timeUntilDeadline(now Timestamp) (time.Duration, bool) {
	if len(s.timers) == 0 {
		return 0, false
	}
	deadline := s.timers[0].deadline
	for _, timer := range s.timers[1:] {
		deadline = min(deadline, timer.deadline)
	}
	if deadline <= now {
		return 0, true
	}
	difference := deadline - now
	if difference > Timestamp(1<<63-1) {
		return time.Duration(1<<63 - 1), true
	}
	return time.Duration(difference), true
}

func (s *effectSupervisor[Message]) startEffect(
	effect Effect[Message],
	scopes []scopeTag,
	continuation effectContinuation,
	now Timestamp,
) {
	if !s.scopesActive(scopes) {
		s.complete(continuation, now)
		return
	}
	switch effect.kind {
	case effectNone:
		s.complete(continuation, now)
	case effectExit:
		s.commands = append(s.commands, runtimeCommand{kind: runtimeCommandExit})
		s.complete(continuation, now)
	case effectFocus:
		s.commands = append(s.commands, runtimeCommand{kind: runtimeCommandFocus, id: effect.id})
		s.complete(continuation, now)
	case effectScrollTo:
		s.commands = append(s.commands, runtimeCommand{
			kind:   runtimeCommandScrollTo,
			id:     effect.id,
			offset: effect.offset,
		})
		s.complete(continuation, now)
	case effectSetClipboard:
		s.commands = append(s.commands, runtimeCommand{
			kind: runtimeCommandSetClipboard, clipboard: effect.clipboard,
		})
		s.complete(continuation, now)
	case effectSuspendTerminal:
		s.startTerminalTask(effect.task, scopes, continuation)
	case effectRun:
		s.startTask(effect.task, scopes, continuation, "", latestTask{}, false, now)
	case effectLatest:
		s.cancelLatest(effect.key, now)
		generation := saturatingAdd64(s.generations[effect.key], 1)
		s.generations[effect.key] = generation
		latest := latestTask{generation: generation}
		latest.id = s.startTask(effect.task, scopes, continuation, effect.key, latest, true, now)
		s.latest[effect.key] = latest
	case effectCancel:
		s.cancelLatest(effect.key, now)
		s.complete(continuation, now)
	case effectScoped:
		if effect.effect == nil {
			s.complete(continuation, now)
			return
		}
		generation := s.scopeGenerations[effect.scope]
		found := false
		for _, tag := range scopes {
			if tag.id == effect.scope {
				found = true
				break
			}
		}
		if !found {
			scopes = append(append([]scopeTag(nil), scopes...), scopeTag{id: effect.scope, generation: generation})
		}
		s.startEffect(*effect.effect, scopes, continuation, now)
	case effectCancelScope:
		s.cancelScope(effect.scope, now)
		s.complete(continuation, now)
	case effectAfter:
		s.timers = append(s.timers, effectTimer[Message]{
			deadline:     now.Add(effect.delay),
			order:        s.nextOrder,
			message:      effect.message,
			scopes:       scopes,
			continuation: continuation,
		})
		s.nextOrder = saturatingAdd64(s.nextOrder, 1)
	case effectBatch:
		if len(effect.effects) == 0 {
			s.complete(continuation, now)
			return
		}
		id := s.identifier()
		s.barriers[id] = &effectBarrier{remaining: len(effect.effects), parent: continuation}
		for _, child := range effect.effects {
			s.startEffect(child, append([]scopeTag(nil), scopes...), effectContinuation{kind: continuationBarrier, id: id}, now)
		}
	case effectSequence:
		if len(effect.effects) == 0 {
			s.complete(continuation, now)
			return
		}
		id := s.identifier()
		s.sequences[id] = &effectSequenceState[Message]{
			remaining: append([]Effect[Message](nil), effect.effects[1:]...),
			scopes:    append([]scopeTag(nil), scopes...),
			parent:    continuation,
		}
		s.startEffect(effect.effects[0], scopes, effectContinuation{kind: continuationSequence, id: id}, now)
	default:
		panic("nagi-tui: invalid effect kind")
	}
}

func (s *effectSupervisor[Message]) startTask(
	task Task[Message],
	scopes []scopeTag,
	continuation effectContinuation,
	latestKey TaskKey,
	latest latestTask,
	hasLatest bool,
	now Timestamp,
) uint64 {
	id := s.identifier()
	ctx, cancel := context.WithCancel(s.parent)
	s.tasks[id] = &effectTaskState[Message]{
		task:            task,
		status:          effectTaskPending,
		context:         ctx,
		cancel:          cancel,
		scopes:          scopes,
		continuation:    continuation,
		hasContinuation: true,
		latestKey:       latestKey,
		latest:          latest,
		hasLatest:       hasLatest,
	}
	s.pending = append(s.pending, id)
	s.spawnAvailable(now)
	return id
}

func (s *effectSupervisor[Message]) startTerminalTask(
	task Task[Message],
	scopes []scopeTag,
	continuation effectContinuation,
) {
	id := s.identifier()
	ctx, cancel := context.WithCancel(s.parent)
	s.terminalTasks[id] = &terminalTaskState[Message]{
		task:            task,
		status:          effectTaskPending,
		context:         ctx,
		cancel:          cancel,
		scopes:          scopes,
		continuation:    continuation,
		hasContinuation: true,
	}
	s.pendingTerminal = append(s.pendingTerminal, id)
}

func (s *effectSupervisor[Message]) spawnAvailable(_ Timestamp) {
	for s.running < s.taskLimit && len(s.pending) > 0 {
		id := s.pending[0]
		s.pending = s.pending[1:]
		state := s.tasks[id]
		if state == nil || state.cancelled {
			continue
		}
		state.status = effectTaskRunning
		s.running++
		task := state.task
		ctx := state.context
		outcomes := s.outcomes
		wake := s.wake
		s.workers.start()
		workers := s.workers
		go func() {
			defer workers.finish()
			message, panicked := runEffectTask(task, ctx)
			outcomes <- effectTaskOutcome[Message]{id: id, message: message, panicked: panicked}
			wake.notify()
		}()
	}
}

func runEffectTask[Message any](task Task[Message], ctx context.Context) (message Message, panicked bool) {
	defer func() {
		if recover() != nil {
			var zero Message
			message = zero
			panicked = true
		}
	}()
	message = task(ctx)
	return message, false
}

func (s *effectSupervisor[Message]) finishTask(outcome effectTaskOutcome[Message], now Timestamp) {
	state := s.tasks[outcome.id]
	if state == nil {
		return
	}
	delete(s.tasks, outcome.id)
	if state.status == effectTaskRunning {
		s.running--
	}
	state.cancel()
	latestMatches := true
	if state.hasLatest {
		current, ok := s.latest[state.latestKey]
		latestMatches = ok && current == (latestTask{id: outcome.id, generation: state.latest.generation})
		if latestMatches {
			delete(s.latest, state.latestKey)
		}
	}
	if outcome.panicked {
		s.diagnostics.taskPanics = saturatingAdd64(s.diagnostics.taskPanics, 1)
		s.notices.push(effectRuntimeNotice(
			RuntimeNoticeEffectPanicked,
			state.latestKey,
			state.latest.generation,
			state.hasLatest,
		))
	} else if !state.cancelled && latestMatches {
		s.ready = append(s.ready, outcome.message)
	} else {
		s.diagnostics.staleResults = saturatingAdd64(s.diagnostics.staleResults, 1)
	}
	if state.hasContinuation {
		state.hasContinuation = false
		s.complete(state.continuation, now)
	}
	s.spawnAvailable(now)
}

func (s *effectSupervisor[Message]) finishTerminalTask(
	id uint64,
	message Message,
	panicked bool,
	now Timestamp,
) {
	state := s.terminalTasks[id]
	if state == nil {
		return
	}
	delete(s.terminalTasks, id)
	state.cancel()
	if panicked {
		s.diagnostics.taskPanics = saturatingAdd64(s.diagnostics.taskPanics, 1)
		s.notices.push(effectRuntimeNotice(RuntimeNoticeEffectPanicked, "", 0, false))
	} else if !state.cancelled {
		s.ready = append(s.ready, message)
	} else {
		s.diagnostics.staleResults = saturatingAdd64(s.diagnostics.staleResults, 1)
	}
	if state.hasContinuation {
		state.hasContinuation = false
		s.complete(state.continuation, now)
	}
}

func (s *effectSupervisor[Message]) cancelLatest(key TaskKey, now Timestamp) {
	if latest, ok := s.latest[key]; ok {
		delete(s.latest, key)
		s.cancelTask(latest.id, now)
	}
}

func (s *effectSupervisor[Message]) cancelTask(id uint64, now Timestamp) {
	state := s.tasks[id]
	if state == nil || state.cancelled {
		return
	}
	state.cancelled = true
	state.cancel()
	s.diagnostics.cancellations = saturatingAdd64(s.diagnostics.cancellations, 1)
	if state.hasLatest {
		key := state.latestKey
		if current, ok := s.latest[key]; ok && current == (latestTask{id: id, generation: state.latest.generation}) {
			delete(s.latest, key)
		}
	}
	if state.status == effectTaskPending {
		delete(s.tasks, id)
	}
	if state.hasContinuation {
		state.hasContinuation = false
		s.complete(state.continuation, now)
	}
	s.spawnAvailable(now)
}

func (s *effectSupervisor[Message]) cancelTerminalTask(id uint64, now Timestamp) {
	state := s.terminalTasks[id]
	if state == nil || state.cancelled {
		return
	}
	state.cancelled = true
	state.cancel()
	s.diagnostics.cancellations = saturatingAdd64(s.diagnostics.cancellations, 1)
	if state.status == effectTaskPending {
		delete(s.terminalTasks, id)
		retained := s.pendingTerminal[:0]
		for _, pendingID := range s.pendingTerminal {
			if pendingID != id {
				retained = append(retained, pendingID)
			}
		}
		s.pendingTerminal = retained
	}
	if state.hasContinuation {
		state.hasContinuation = false
		s.complete(state.continuation, now)
	}
}

func (s *effectSupervisor[Message]) cancelScope(scope ScopeID, now Timestamp) {
	generation := s.scopeGenerations[scope]
	s.scopeGenerations[scope] = saturatingAdd64(generation, 1)
	var tasks []uint64
	for id, task := range s.tasks {
		if containsScopeTag(task.scopes, scope, generation) {
			tasks = append(tasks, id)
		}
	}
	for _, id := range tasks {
		s.cancelTask(id, now)
	}

	var terminalTasks []uint64
	for id, task := range s.terminalTasks {
		if containsScopeTag(task.scopes, scope, generation) {
			terminalTasks = append(terminalTasks, id)
		}
	}
	for _, id := range terminalTasks {
		s.cancelTerminalTask(id, now)
	}

	retained := make([]effectTimer[Message], 0, len(s.timers))
	var cancelled []effectContinuation
	for _, timer := range s.timers {
		if containsScopeTag(timer.scopes, scope, generation) {
			cancelled = append(cancelled, timer.continuation)
			s.diagnostics.cancellations = saturatingAdd64(s.diagnostics.cancellations, 1)
		} else {
			retained = append(retained, timer)
		}
	}
	s.timers = retained
	for _, continuation := range cancelled {
		s.complete(continuation, now)
	}
}

func (s *effectSupervisor[Message]) pollTimers(now Timestamp) {
	for {
		retained := make([]effectTimer[Message], 0, len(s.timers))
		var due []effectTimer[Message]
		for _, timer := range s.timers {
			if timer.deadline <= now {
				due = append(due, timer)
			} else {
				retained = append(retained, timer)
			}
		}
		s.timers = retained
		if len(due) == 0 {
			return
		}
		sort.SliceStable(due, func(i, j int) bool {
			if due[i].deadline != due[j].deadline {
				return due[i].deadline < due[j].deadline
			}
			return due[i].order < due[j].order
		})
		for _, timer := range due {
			if s.scopesActive(timer.scopes) {
				s.ready = append(s.ready, timer.message)
			}
			s.complete(timer.continuation, now)
		}
	}
}

func (s *effectSupervisor[Message]) complete(continuation effectContinuation, now Timestamp) {
	for {
		switch continuation.kind {
		case continuationNone:
			return
		case continuationBarrier:
			barrier := s.barriers[continuation.id]
			if barrier == nil {
				return
			}
			barrier.remaining--
			if barrier.remaining > 0 {
				return
			}
			delete(s.barriers, continuation.id)
			continuation = barrier.parent
		case continuationSequence:
			sequence := s.sequences[continuation.id]
			if sequence == nil {
				return
			}
			if len(sequence.remaining) == 0 {
				delete(s.sequences, continuation.id)
				continuation = sequence.parent
				continue
			}
			next := sequence.remaining[0]
			sequence.remaining = sequence.remaining[1:]
			s.startEffect(next, append([]scopeTag(nil), sequence.scopes...), continuation, now)
			return
		default:
			panic("nagi-tui: invalid effect continuation")
		}
	}
}

func (s *effectSupervisor[Message]) scopesActive(scopes []scopeTag) bool {
	for _, tag := range scopes {
		if s.scopeGenerations[tag.id] != tag.generation {
			return false
		}
	}
	return true
}

func (s *effectSupervisor[Message]) identifier() uint64 {
	id := s.nextIdentifier
	s.nextIdentifier = saturatingAdd64(s.nextIdentifier, 1)
	return id
}

func (s *effectSupervisor[Message]) close() {
	if s.closed {
		return
	}
	s.closed = true
	for _, task := range s.tasks {
		task.cancelled = true
		task.cancel()
	}
	for _, task := range s.terminalTasks {
		task.cancelled = true
		task.cancel()
	}
	s.tasks = make(map[uint64]*effectTaskState[Message])
	s.terminalTasks = make(map[uint64]*terminalTaskState[Message])
	s.pending = nil
	s.pendingTerminal = nil
	s.timers = nil
	s.commands = nil
	s.sequences = make(map[uint64]*effectSequenceState[Message])
	s.barriers = make(map[uint64]*effectBarrier)
}

func containsScopeTag(scopes []scopeTag, scope ScopeID, generation uint64) bool {
	for _, tag := range scopes {
		if tag.id == scope && tag.generation == generation {
			return true
		}
	}
	return false
}
