package embedded

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func newBackend(t *testing.T) *Backend {
	t.Helper()
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newKey(t *testing.T) (ssh.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshPublic, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	return sshPublic, private
}

func TestAddListSignRemoveAndRemoveAll(t *testing.T) {
	b := newBackend(t)
	public, private := newKey(t)
	if err := b.Add(agent.AddedKey{PrivateKey: private, Comment: "first"}); err != nil {
		t.Fatal(err)
	}
	keys, err := b.List()
	if err != nil || len(keys) != 1 {
		t.Fatalf("keys=%d err=%v", len(keys), err)
	}
	if keys[0].Comment != "first" {
		t.Fatalf("comment=%q", keys[0].Comment)
	}
	message := []byte("embedded backend signing test")
	signature, err := b.Sign(public, message)
	if err != nil {
		t.Fatal(err)
	}
	if err := public.Verify(message, signature); err != nil {
		t.Fatalf("verify signature: %v", err)
	}
	if err := b.Remove(public); err != nil {
		t.Fatal(err)
	}
	keys, err = b.List()
	if err != nil || len(keys) != 0 {
		t.Fatalf("after remove keys=%d err=%v", len(keys), err)
	}
	if err := b.Add(agent.AddedKey{PrivateKey: private}); err != nil {
		t.Fatal(err)
	}
	if err := b.RemoveAll(); err != nil {
		t.Fatal(err)
	}
	keys, err = b.List()
	if err != nil || len(keys) != 0 {
		t.Fatalf("after remove all keys=%d err=%v", len(keys), err)
	}
}

func TestLockAndUnlock(t *testing.T) {
	b := newBackend(t)
	public, private := newKey(t)
	if err := b.Add(agent.AddedKey{PrivateKey: private}); err != nil {
		t.Fatal(err)
	}
	passphrase := []byte("test passphrase")
	if err := b.Lock(passphrase); err != nil {
		t.Fatal(err)
	}
	keys, err := b.List()
	if err != nil || len(keys) != 0 {
		t.Fatalf("locked list keys=%d err=%v", len(keys), err)
	}
	if _, err := b.Sign(public, []byte("message")); err == nil {
		t.Fatal("expected signing to fail while locked")
	}
	if err := b.Unlock([]byte("wrong")); err == nil {
		t.Fatal("expected incorrect passphrase to fail")
	}
	if err := b.Unlock(passphrase); err != nil {
		t.Fatal(err)
	}
	keys, err = b.List()
	if err != nil || len(keys) != 1 {
		t.Fatalf("unlocked list keys=%d err=%v", len(keys), err)
	}
}

func TestRSASHA2SignatureFlags(t *testing.T) {
	b := newBackend(t)
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	public, err := ssh.NewPublicKey(&private.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Add(agent.AddedKey{PrivateKey: private}); err != nil {
		t.Fatal(err)
	}
	message := []byte("rsa sha2 signing test")
	for _, test := range []struct {
		flags agent.SignatureFlags
		algo  string
	}{
		{flags: agent.SignatureFlagRsaSha256, algo: ssh.KeyAlgoRSASHA256},
		{flags: agent.SignatureFlagRsaSha512, algo: ssh.KeyAlgoRSASHA512},
	} {
		signature, err := b.SignWithFlags(public, message, test.flags)
		if err != nil {
			t.Fatal(err)
		}
		if signature.Format != test.algo {
			t.Fatalf("signature format=%q, want %q", signature.Format, test.algo)
		}
		if err := public.Verify(message, signature); err != nil {
			t.Fatalf("verify %s signature: %v", test.algo, err)
		}
	}
}

func TestLifetimeExpiry(t *testing.T) {
	b := newBackend(t)
	_, private := newKey(t)
	if err := b.Add(agent.AddedKey{PrivateKey: private, LifetimeSecs: 1}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		keys, err := b.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(keys) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("key did not expire")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestConfirmBeforeUseIsRejected(t *testing.T) {
	b := newBackend(t)
	_, private := newKey(t)
	err := b.Add(agent.AddedKey{PrivateKey: private, ConfirmBeforeUse: true})
	if !errors.Is(err, ErrConfirmBeforeUseUnsupported) {
		t.Fatalf("err=%v", err)
	}
	keys, listErr := b.List()
	if listErr != nil || len(keys) != 0 {
		t.Fatalf("rejected key was retained: keys=%d err=%v", len(keys), listErr)
	}
}

func TestBackendsDoNotShareKeys(t *testing.T) {
	first := newBackend(t)
	_, private := newKey(t)
	if err := first.Add(agent.AddedKey{PrivateKey: private}); err != nil {
		t.Fatal(err)
	}
	second := newBackend(t)
	keys, err := second.List()
	if err != nil || len(keys) != 0 {
		t.Fatalf("new backend keys=%d err=%v", len(keys), err)
	}
}
