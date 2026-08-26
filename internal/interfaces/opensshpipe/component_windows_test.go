//go:build windows

package opensshpipe

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/masahide/OmniSSHAgent/internal/app"
	"github.com/masahide/OmniSSHAgent/internal/backend/embedded"
	"github.com/masahide/OmniSSHAgent/internal/interfaces"
	"github.com/masahide/OmniSSHAgent/internal/testutil"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/sys/windows"
)

var testPipeSequence atomic.Uint64

type stateSink struct{ state app.State }

func (s *stateSink) SetState(state app.State) { s.state = state }

func uniquePipePath() string {
	return fmt.Sprintf(`\\.\pipe\OmniSSHAgent-OpenSSH-Test-%d-%d`, os.Getpid(), testPipeSequence.Add(1))
}

func testBackend(t *testing.T) *embedded.Backend {
	t.Helper()
	b, err := embedded.New()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type runningComponent struct {
	cancel context.CancelFunc
	done   <-chan error
	once   sync.Once
}

func startComponent(t *testing.T, path string, b agent.ExtendedAgent) *runningComponent {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c := newWithPipePath(b, path, slog.New(slog.NewTextHandler(io.Discard, nil)))
	done := make(chan error, 1)
	go func() { done <- c.Start(ctx) }()
	select {
	case err := <-c.Ready():
		if err != nil {
			cancel()
			t.Fatalf("start component: %v", err)
		}
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("component did not report readiness")
	}
	running := &runningComponent{cancel: cancel, done: done}
	t.Cleanup(func() { running.stop(t) })
	return running
}

func (r *runningComponent) stop(t *testing.T) {
	t.Helper()
	r.once.Do(func() {
		r.cancel()
		select {
		case err := <-r.done:
			if err != nil {
				t.Errorf("stop component: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("component did not stop")
		}
	})
}

func dialAgent(t *testing.T, path string) (agent.ExtendedAgent, io.Closer) {
	t.Helper()
	conn, err := winio.DialPipe(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	return agent.NewClient(conn), conn
}

func TestPipeSecurityDescriptorAllowsOnlySystemAndCurrentUser(t *testing.T) {
	sddl, err := pipeSecurityDescriptor()
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	want := "D:P(A;;GA;;;SY)(A;;GA;;;" + user.User.Sid.String() + ")"
	if sddl != want {
		t.Fatalf("SDDL=%q, want %q", sddl, want)
	}
	if _, err := windows.SecurityDescriptorFromString(sddl); err != nil {
		t.Fatalf("invalid SDDL: %v", err)
	}
}

func TestAgentOperationsThroughNamedPipe(t *testing.T) {
	path := uniquePipePath()
	startComponent(t, path, testBackend(t))
	client, conn := dialAgent(t, path)
	defer conn.Close()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshPublic, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Add(agent.AddedKey{PrivateKey: private, Comment: "pipe-test"}); err != nil {
		t.Fatal(err)
	}
	keys, err := client.List()
	if err != nil || len(keys) != 1 {
		t.Fatalf("keys=%d err=%v", len(keys), err)
	}
	message := []byte("named pipe signing test")
	signature, err := client.Sign(sshPublic, message)
	if err != nil {
		t.Fatal(err)
	}
	if err := sshPublic.Verify(message, signature); err != nil {
		t.Fatalf("verify signature: %v", err)
	}
	if err := client.Remove(sshPublic); err != nil {
		t.Fatal(err)
	}
	keys, err = client.List()
	if err != nil || len(keys) != 0 {
		t.Fatalf("after remove keys=%d err=%v", len(keys), err)
	}
}

func TestMultipleSimultaneousClients(t *testing.T) {
	path := uniquePipePath()
	b := testBackend(t)
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Add(agent.AddedKey{PrivateKey: private}); err != nil {
		t.Fatal(err)
	}
	startComponent(t, path, b)

	const clientCount = 12
	release := make(chan struct{})
	errs := make(chan error, clientCount)
	var connected sync.WaitGroup
	var wg sync.WaitGroup
	connected.Add(clientCount)
	for range clientCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := winio.DialPipe(path, nil)
			connected.Done()
			if err != nil {
				errs <- err
				return
			}
			defer conn.Close()
			<-release
			keys, err := agent.NewClient(conn).List()
			if err == nil && len(keys) != 1 {
				err = fmt.Errorf("keys=%d", len(keys))
			}
			errs <- err
		}()
	}
	connected.Wait()
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestExistingPipeConflictIsReported(t *testing.T) {
	path := uniquePipePath()
	sddl, err := pipeSecurityDescriptor()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := winio.ListenPipe(path, &winio.PipeConfig{SecurityDescriptor: sddl})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newWithPipePath(testBackend(t), path, nil)
	done := make(chan error, 1)
	go func() { done <- c.Start(ctx) }()
	select {
	case err := <-c.Ready():
		if err == nil {
			t.Fatal("expected pipe conflict")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("component did not report pipe conflict")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected startup error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("component did not stop after pipe conflict")
	}
}

func TestPipeConflictDegradesApplicationWithoutStoppingOtherInterfaces(t *testing.T) {
	path := uniquePipePath()
	sddl, err := pipeSecurityDescriptor()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := winio.ListenPipe(path, &winio.PipeConfig{SecurityDescriptor: sddl})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()

	openSSH := newWithPipePath(testBackend(t), path, nil)
	pageant := &testutil.Component{ComponentName: "pageant"}
	cygwin := &testutil.Component{ComponentName: "cygwin"}
	sink := &stateSink{}
	application := app.New(
		[]interfaces.Component{openSSH, pageant, cygwin},
		sink,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	application.Run(context.Background())
	if sink.state != app.StateDegraded {
		t.Fatalf("state=%s", sink.state)
	}
	if !pageant.Started.Load() || !cygwin.Started.Load() {
		t.Fatalf("other interfaces did not remain running: pageant=%v cygwin=%v", pageant.Started.Load(), cygwin.Started.Load())
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := application.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownClosesIdleAndPartialClients(t *testing.T) {
	path := uniquePipePath()
	running := startComponent(t, path, testBackend(t))
	idle, err := winio.DialPipe(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close()
	partial, err := winio.DialPipe(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer partial.Close()
	if _, err := partial.Write([]byte{0, 0, 0, 32, 11}); err != nil {
		t.Fatal(err)
	}
	running.stop(t)
}
