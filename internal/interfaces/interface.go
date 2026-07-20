package interfaces

import "context"

// Component is a long-running compatibility interface.
// Start returns only after startup fails or the component stops.
type Component interface {
	Name() string
	Start(context.Context) error
}

// ReadyComponent reports whether its externally visible resources were
// prepared successfully. The channel yields exactly one startup result.
type ReadyComponent interface {
	Component
	Ready() <-chan error
}
