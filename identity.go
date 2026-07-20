package tui

// NodeID is an application-defined stable identity for an interactive or
// stateful node
type NodeID string

// NewNodeID returns an identifier from an application-defined stable key
func NewNodeID(value string) NodeID {
	return NodeID(value)
}

// String returns the application-defined key
func (id NodeID) String() string {
	return string(id)
}
