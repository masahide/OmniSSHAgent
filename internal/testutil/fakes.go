package testutil

import (
	"context"
	"sync"
	"sync/atomic"
)

type Component struct {
	ComponentName string
	StartError    error
	Started       atomic.Bool
	readyInit     sync.Once
	readySignal   sync.Once
	ready         chan error
}

func (c *Component) Name() string { return c.ComponentName }
func (c *Component) Ready() <-chan error {
	c.readyInit.Do(func() { c.ready = make(chan error, 1) })
	return c.ready
}
func (c *Component) Start(ctx context.Context) error {
	c.Started.Store(true)
	ready := c.Ready()
	c.readySignal.Do(func() { c.ready <- c.StartError })
	if c.StartError != nil {
		return c.StartError
	}
	<-ctx.Done()
	_ = ready
	return nil
}
