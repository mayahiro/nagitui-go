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
	route  []NodeID
	groups []*resolvedActionGroup[Message]
}

type resolvedActionGroup[Message any] struct {
	actions  []Action[Message]
	resolved ResolvedActions
}

func resolveActionRoute[Message any](
	route []NodeID,
	actions *actionIndex[Message],
) (*resolvedActionRoute[Message], error) {
	scopes := scopesForRoute(actions, route)
	allowed := len(route)
	for index, id := range route {
		record, ok := actions.record(id)
		if ok && record.scope != nil && record.scope.propagation == KeyScopeStopAtScope {
			allowed = index + 1
			break
		}
	}
	resolved := &resolvedActionRoute[Message]{
		route:  append([]NodeID(nil), route...),
		groups: make([]*resolvedActionGroup[Message], len(route)),
	}
	// Records follow semantic tree order, fixing which conflict is returned
	// when more than one active group is invalid
	for recordIndex := range actions.records {
		owner := &actions.records[recordIndex]
		if len(owner.actions) == 0 {
			continue
		}
		routeIndex := indexNodeID(route[:allowed], owner.id)
		if routeIndex < 0 {
			continue
		}
		group, err := ResolveActions(owner.id, owner.descriptors, scopes)
		if err != nil {
			return nil, err
		}
		resolved.groups[routeIndex] = &resolvedActionGroup[Message]{
			actions:  owner.actions,
			resolved: group,
		}
	}
	return resolved, nil
}

func (r *resolvedActionRoute[Message]) matchesRoute(route []NodeID) bool {
	if r == nil || len(r.route) != len(route) {
		return false
	}
	for index := range route {
		if r.route[index] != route[index] {
			return false
		}
	}
	return true
}

func (r *resolvedActionRoute[Message]) matchEvent(
	routeIndex int,
	event vt.Event,
) (EventResult[Message], bool) {
	if routeIndex < 0 || routeIndex >= len(r.groups) || r.groups[routeIndex] == nil {
		return EventResult[Message]{}, false
	}
	stroke, ok := KeyStrokeFromEvent(event)
	if !ok {
		return EventResult[Message]{}, false
	}
	group := r.groups[routeIndex]
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

func (r *resolvedActionRoute[Message]) actionGroups() []ResolvedActions {
	groups := make([]ResolvedActions, 0, len(r.groups))
	for _, group := range r.groups {
		if group != nil {
			groups = append(groups, group.resolved)
		}
	}
	return groups
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
