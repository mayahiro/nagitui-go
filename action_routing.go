package tui

import "github.com/mayahiro/nagi-go/vt"

type nodeKeyInteraction[Message any] struct {
	actions []Action[Message]
	scope   *nodeKeyScope
}

type nodeKeyScope struct {
	keyMap      KeyMap
	propagation KeyScopePropagation
}

type actionNode[Message any] struct {
	id          NodeID
	actions     []Action[Message]
	descriptors []ActionDescriptor
	scope       *nodeKeyScope
}

type actionIndex[Message any] struct {
	records    []actionNode[Message]
	byID       map[NodeID]int
	hasActions bool
}

func (a *actionIndex[Message]) reset() {
	clear(a.records)
	a.records = a.records[:0]
	clear(a.byID)
	a.hasActions = false
}

func (a *actionIndex[Message]) register(id NodeID, interaction *nodeKeyInteraction[Message]) {
	if interaction == nil || len(interaction.actions) == 0 && interaction.scope == nil {
		return
	}
	a.hasActions = a.hasActions || len(interaction.actions) > 0
	if a.byID == nil {
		a.byID = make(map[NodeID]int)
	}
	a.byID[id] = len(a.records)
	a.records = append(a.records, actionNode[Message]{
		id:          id,
		actions:     interaction.actions,
		descriptors: actionDescriptors(interaction.actions),
		scope:       interaction.scope,
	})
}

func (a *actionIndex[Message]) record(id NodeID) (*actionNode[Message], bool) {
	index, ok := a.byID[id]
	if !ok {
		return nil, false
	}
	return &a.records[index], true
}

func validateActionOwners[Message any](tree *treeIndex, actions *actionIndex[Message]) error {
	if !actions.hasActions {
		return nil
	}
	for index := range actions.records {
		owner := &actions.records[index]
		if len(owner.actions) == 0 {
			continue
		}
		route := tree.rawRoute(owner.id, true)
		scopes := scopesForRoute(actions, route)
		if _, err := ResolveActions(owner.id, owner.descriptors, scopes); err != nil {
			return err
		}
	}
	return nil
}

type resolvedActionRoute[Message any] struct {
	route         []NodeID
	focusOwner    NodeID
	hasFocusOwner bool
	groups        []resolvedRouteGroups[Message]
}

type resolvedRouteGroups[Message any] struct {
	declared    resolvedActionGroup[Message]
	hasDeclared bool
	core        resolvedCoreActionGroup
	hasCore     bool
}

type resolvedActionGroup[Message any] struct {
	actions  []Action[Message]
	resolved ResolvedActions
}

type resolvedCoreActionGroup struct {
	actions  []coreAction
	resolved ResolvedActions
}

func (r *resolvedActionRoute[Message]) resolveInto(
	route []NodeID,
	actions *actionIndex[Message],
	tree *treeIndex,
	focusOwner NodeID,
	hasFocusOwner bool,
) error {
	r.route = append(r.route[:0], route...)
	return r.resolveGroups(actions, tree, focusOwner, hasFocusOwner)
}

func (r *resolvedActionRoute[Message]) resolveTreeRouteInto(
	target NodeID,
	hasTarget bool,
	actions *actionIndex[Message],
	tree *treeIndex,
	focusOwner NodeID,
	hasFocusOwner bool,
) (bool, error) {
	r.route = tree.routeInto(target, hasTarget, r.route)
	if !routeNeedsActionResolution(r.route, actions, tree, focusOwner, hasFocusOwner) {
		r.focusOwner = ""
		r.hasFocusOwner = false
		r.clearGroups()
		return false, nil
	}
	if err := r.resolveGroups(actions, tree, focusOwner, hasFocusOwner); err != nil {
		return false, err
	}
	return true, nil
}

func (r *resolvedActionRoute[Message]) resolveGroups(
	actions *actionIndex[Message],
	tree *treeIndex,
	focusOwner NodeID,
	hasFocusOwner bool,
) error {
	scopes := scopesForRoute(actions, r.route)
	allowed := allowedActionRouteLength(r.route, actions)
	r.clearGroups()
	if cap(r.groups) < len(r.route) {
		r.groups = make([]resolvedRouteGroups[Message], len(r.route))
	} else {
		r.groups = r.groups[:len(r.route)]
	}
	// Records follow semantic tree order, fixing which conflict is returned
	// when more than one active group is invalid
	for recordIndex := range actions.records {
		owner := &actions.records[recordIndex]
		if len(owner.actions) == 0 {
			continue
		}
		routeIndex := indexNodeID(r.route[:allowed], owner.id)
		if routeIndex < 0 {
			continue
		}
		group, err := ResolveActions(owner.id, owner.descriptors, scopes)
		if err != nil {
			return err
		}
		r.groups[routeIndex].declared = resolvedActionGroup[Message]{
			actions:  owner.actions,
			resolved: group,
		}
		r.groups[routeIndex].hasDeclared = true
	}
	for routeIndex, owner := range r.route[:allowed] {
		record, _ := tree.record(owner)
		scrollAxis, hasScroll := record.kind.scrollAxis()
		group, ok := coreActionGroupFor(
			hasFocusOwner && owner == focusOwner,
			scrollAxis,
			hasScroll,
		)
		if !ok {
			continue
		}
		resolvedGroup, err := resolveCoreActionGroup(owner, group, scopes)
		if err != nil {
			return err
		}
		r.groups[routeIndex].core = resolvedCoreActionGroup{
			actions:  group.actions,
			resolved: resolvedGroup,
		}
		r.groups[routeIndex].hasCore = true
	}
	r.focusOwner = focusOwner
	r.hasFocusOwner = hasFocusOwner
	return nil
}

func (r *resolvedActionRoute[Message]) clearGroups() {
	for index := range r.groups {
		r.groups[index] = resolvedRouteGroups[Message]{}
	}
	r.groups = r.groups[:0]
}

func (r *resolvedActionRoute[Message]) matchesRoute(
	route []NodeID,
	focusOwner NodeID,
	hasFocusOwner bool,
) bool {
	if r == nil || len(r.route) != len(route) ||
		r.hasFocusOwner != hasFocusOwner || hasFocusOwner && r.focusOwner != focusOwner {
		return false
	}
	for index := range route {
		if r.route[index] != route[index] {
			return false
		}
	}
	return true
}

func (r *resolvedActionRoute[Message]) matchDeclaredEvent(
	routeIndex int,
	event vt.Event,
) (EventResult[Message], bool) {
	if routeIndex < 0 || routeIndex >= len(r.groups) || !r.groups[routeIndex].hasDeclared {
		return EventResult[Message]{}, false
	}
	stroke, ok := KeyStrokeFromEvent(event)
	if !ok {
		return EventResult[Message]{}, false
	}
	group := &r.groups[routeIndex].declared
	for actionIndex, resolved := range group.resolved.actions {
		matched := false
		for _, binding := range resolved.bindings {
			if binding.Matches(event) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		switch resolved.availability {
		case ActionEnabled:
			return group.actions[actionIndex].invoke(ActionEvent{
				action: resolved.id,
				stroke: stroke,
			}), true
		case ActionDisabledConsume:
			return ConsumeResult[Message](), true
		case ActionDisabledPassThrough:
			continue
		}
	}
	return EventResult[Message]{}, false
}

type coreActionMatch uint8

const (
	coreActionNone coreActionMatch = iota
	coreActionConsume
	coreActionInvoke
)

func (r *resolvedActionRoute[Message]) matchCoreEvent(
	routeIndex int,
	event vt.Event,
) (coreAction, coreActionMatch) {
	if routeIndex < 0 || routeIndex >= len(r.groups) || !r.groups[routeIndex].hasCore {
		return 0, coreActionNone
	}
	group := &r.groups[routeIndex].core
	for actionIndex, resolved := range group.resolved.actions {
		matched := false
		for _, binding := range resolved.bindings {
			if binding.Matches(event) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		switch resolved.availability {
		case ActionEnabled:
			return group.actions[actionIndex], coreActionInvoke
		case ActionDisabledConsume:
			return 0, coreActionConsume
		case ActionDisabledPassThrough:
			continue
		}
	}
	return 0, coreActionNone
}

func (r *resolvedActionRoute[Message]) actionGroups() []ResolvedActions {
	groups := make([]ResolvedActions, 0, len(r.groups)*2)
	for _, group := range r.groups {
		if group.hasDeclared {
			groups = append(groups, group.declared.resolved)
		}
		if group.hasCore {
			groups = append(groups, group.core.resolved)
		}
	}
	return groups
}

func routeNeedsActionResolution[Message any](
	route []NodeID,
	actions *actionIndex[Message],
	tree *treeIndex,
	focusOwner NodeID,
	hasFocusOwner bool,
) bool {
	if actions.hasActions {
		return true
	}
	for _, id := range route[:allowedActionRouteLength(route, actions)] {
		record, _ := tree.record(id)
		if hasFocusOwner && id == focusOwner || record.kind.isScrollViewport() {
			return true
		}
	}
	return false
}

func allowedActionRouteLength[Message any](route []NodeID, actions *actionIndex[Message]) int {
	for index, id := range route {
		record, ok := actions.record(id)
		if ok && record.scope != nil && record.scope.propagation == KeyScopeStopAtScope {
			return index + 1
		}
	}
	return len(route)
}

func scopesForRoute[Message any](actions *actionIndex[Message], targetToRoot []NodeID) []KeyScope {
	var scopes []KeyScope
	for index := len(targetToRoot) - 1; index >= 0; index-- {
		record, ok := actions.record(targetToRoot[index])
		if !ok || record.scope == nil {
			continue
		}
		scopes = append(scopes, NewKeyScope(record.id, record.scope.keyMap).
			WithPropagation(record.scope.propagation))
	}
	return scopes
}

func actionDescriptors[Message any](actions []Action[Message]) []ActionDescriptor {
	descriptors := make([]ActionDescriptor, len(actions))
	for index := range actions {
		descriptors[index] = actions[index].descriptor
	}
	return descriptors
}

func indexNodeID(ids []NodeID, target NodeID) int {
	for index, id := range ids {
		if id == target {
			return index
		}
	}
	return -1
}
