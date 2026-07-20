package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/masahide/OmniSSHAgent/internal/interfaces"
	"github.com/masahide/OmniSSHAgent/internal/testutil"
)

type sink struct{ state State }

func (s *sink) SetState(state State) { s.state = state }

type stubbornComponent struct{}

func (stubbornComponent) Name() string                { return "stubborn" }
func (stubbornComponent) Start(context.Context) error { select {} }

type panicComponent struct{}

func (panicComponent) Name() string                { return "panic" }
func (panicComponent) Start(context.Context) error { panic("boom") }
func (panicComponent) Ready() <-chan error         { return make(chan error) }

type orderedStopComponent struct {
	name      string
	delay     time.Duration
	ready     chan error
	stopped   chan string
	readyOnce sync.Once
}

func newOrderedStopComponent(name string, delay time.Duration, stopped chan string) *orderedStopComponent {
	return &orderedStopComponent{name: name, delay: delay, ready: make(chan error, 1), stopped: stopped}
}

func (c *orderedStopComponent) Name() string        { return c.name }
func (c *orderedStopComponent) Ready() <-chan error { return c.ready }
func (c *orderedStopComponent) Start(ctx context.Context) error {
	c.readyOnce.Do(func() { c.ready <- nil })
	<-ctx.Done()
	time.Sleep(c.delay)
	c.stopped <- c.name
	return nil
}

func TestApplicationDegradedAndShutdown(t *testing.T) {
	stateSink := &sink{}
	components := []interfaces.Component{
		&testutil.Component{ComponentName: "ok"},
		&testutil.Component{ComponentName: "bad", StartError: errors.New("conflict")},
	}
	a := New(components, stateSink, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.Run(context.Background())
	if a.State() != StateDegraded {
		t.Fatal(a.State())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownIsIdempotent(t *testing.T) {
	a := New([]interfaces.Component{&testutil.Component{ComponentName: "ok"}}, &sink{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.Run(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := a.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownTimeout(t *testing.T) {
	a := New([]interfaces.Component{stubbornComponent{}}, &sink{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.Run(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := a.Shutdown(ctx); err == nil {
		t.Fatal("expected timeout")
	}
}

func TestComponentPanicBecomesDegraded(t *testing.T) {
	a := New([]interfaces.Component{panicComponent{}}, &sink{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.Run(context.Background())
	if a.State() != StateDegraded {
		t.Fatal(a.State())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestConfigurationErrorDoesNotStartComponents(t *testing.T) {
	component := &testutil.Component{ComponentName: "must-not-start"}
	stateSink := &sink{}
	a := New([]interfaces.Component{component}, stateSink, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.SetConfigurationError()
	if component.Started.Load() {
		t.Fatal("component started in configuration error mode")
	}
	if a.State() != StateConfigurationError || stateSink.state != StateConfigurationError {
		t.Fatalf("state=%s sink=%s", a.State(), stateSink.state)
	}
}

func TestPageantCygwinTrayStopInAnyOrder(t *testing.T) {
	orders := [][]string{
		{"pageant", "cygwin", "tray"},
		{"pageant", "tray", "cygwin"},
		{"cygwin", "pageant", "tray"},
		{"cygwin", "tray", "pageant"},
		{"tray", "pageant", "cygwin"},
		{"tray", "cygwin", "pageant"},
	}
	for _, order := range orders {
		t.Run(order[0]+"-"+order[1]+"-"+order[2], func(t *testing.T) {
			stopped := make(chan string, 3)
			delayByName := map[string]time.Duration{
				order[0]: 0,
				order[1]: 10 * time.Millisecond,
				order[2]: 20 * time.Millisecond,
			}
			components := []interfaces.Component{
				newOrderedStopComponent("pageant", delayByName["pageant"], stopped),
				newOrderedStopComponent("cygwin", delayByName["cygwin"], stopped),
				newOrderedStopComponent("tray", delayByName["tray"], stopped),
			}
			application := New(components, &sink{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			application.Run(context.Background())
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := application.Shutdown(ctx); err != nil {
				t.Fatal(err)
			}
			var actual []string
			for range 3 {
				actual = append(actual, <-stopped)
			}
			for index, name := range order {
				if actual[index] != name {
					t.Fatalf("stop order=%v, want %v", actual, order)
				}
			}
		})
	}
}
