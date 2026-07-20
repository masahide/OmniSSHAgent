package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/masahide/OmniSSHAgent/internal/interfaces"
)

type StateSink interface {
	SetState(State)
}

type Application struct {
	components []interfaces.Component
	sink       StateSink
	logger     *slog.Logger
	mu         sync.RWMutex
	state      State
	statuses   []ComponentStatus
	cancel     context.CancelFunc
	done       chan struct{}
	shutdown   sync.Once
}

func New(components []interfaces.Component, sink StateSink, logger *slog.Logger) *Application {
	return &Application{components: components, sink: sink, logger: logger, state: StateDegraded, done: make(chan struct{})}
}

func (a *Application) State() State {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

func (a *Application) Run(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	a.cancel = cancel
	var wg sync.WaitGroup
	results := make(chan ComponentStatus, len(a.components))
	for _, component := range a.components {
		component := component
		wg.Add(1)
		go func() {
			defer wg.Done()
			stopped := make(chan error, 1)
			go func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						stopped <- fmt.Errorf("component %s panicked: %v", component.Name(), recovered)
					}
				}()
				stopped <- component.Start(ctx)
			}()
			if ready, ok := component.(interfaces.ReadyComponent); ok {
				select {
				case err := <-ready.Ready():
					results <- ComponentStatus{Name: component.Name(), Enabled: true, Running: err == nil, Error: err}
				case err := <-stopped:
					results <- ComponentStatus{Name: component.Name(), Enabled: true, Error: err}
					return
				case <-ctx.Done():
					results <- ComponentStatus{Name: component.Name(), Enabled: true, Error: ctx.Err()}
				}
			} else {
				results <- ComponentStatus{Name: component.Name(), Enabled: true, Running: true}
			}
			<-stopped
		}()
	}
	for range a.components {
		status := <-results
		a.statuses = append(a.statuses, status)
		if status.Error != nil && a.logger != nil {
			a.logger.Warn("component startup failed", "component", status.Name, "error", status.Error)
		}
	}
	a.setState(Aggregate(nil, a.statuses))
	go func() { wg.Wait(); close(a.done) }()
}

func (a *Application) SetConfigurationError() { a.setState(StateConfigurationError) }

func (a *Application) setState(state State) {
	a.mu.Lock()
	a.state = state
	a.mu.Unlock()
	if a.logger != nil {
		a.logger.Info("application state changed", "state", state)
	}
	if a.sink != nil {
		a.sink.SetState(state)
	}
}

func (a *Application) Shutdown(ctx context.Context) error {
	a.shutdown.Do(func() {
		if a.cancel != nil {
			a.cancel()
		}
	})
	select {
	case <-a.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown components: %w", ctx.Err())
	}
}
