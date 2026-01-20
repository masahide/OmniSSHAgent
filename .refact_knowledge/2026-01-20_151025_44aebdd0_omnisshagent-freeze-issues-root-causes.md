---
id: "b552d667-1677-48dd-b09c-9ae19a4e4101"
title: "OmniSSHAgent Freeze Bugs: Root Causes & Fixes"
kind: code
created: 2026-01-20
updated: 2026-01-20
review_after: 2026-04-20
status: deprecated
tags: ["bug-fix", "bugfix", "critical", "deadlock", "error-handling", "freeze", "go", "infinite-recursion", "keyring", "nil-deref", "nil-pointer", "recursion", "ssh-agent"]
filenames: ["pkg/pageant/pageant.go", "pkg/sshutil/sshutil.go"]
superseded_by: "8b634699-9518-4664-9563-10a044e3e208"
deprecated_at: 2026-01-20
---


> ⚠️ **DEPRECATED**: Covers the same topic of OmniSSHAgent freeze bugs with overlapping content on the 3 immediate freeze bugs (e.g., infinite recursion in sshutil.go:385); new doc is more complete/updated with 6 bugs fixed total, including the original 3 plus 3 new resource leak fixes, making it a superset.

# OmniSSHAgent Freeze Issues - Root Causes and Fixes

## Summary
Found and fixed **3 critical bugs** causing the SSH agent to freeze at runtime, particularly when queried via `ssh-add` on Unix sockets.

## Bug #1: Infinite Recursion in KeyRing.Signers()
**File**: `pkg/sshutil/sshutil.go:385` (FIXED)
**Severity**: CRITICAL - Stack overflow

### Problem
```go
func (k *KeyRing) Signers() ([]ssh.Signer, error) {
	k.notice("Signers", nil)
	defer k.notice("Signers", nil)
	return k.Signers()  // ❌ CALLS ITSELF - INFINITE RECURSION!
}
```

### Root Cause
The method calls itself recursively with no base case, causing immediate stack overflow when invoked.

### Fix
Changed to delegate to the underlying keyring:
```go
return k.keyring.Signers()
```

---

## Bug #2: Inverted Error Logic in Pageant Agent
**File**: `pkg/pageant/pageant.go:142-146` (FIXED)
**Severity**: CRITICAL - Incorrect error handling

### Problem
```go
err := agent.ServeAgent(a, struct {
	io.Reader
	io.Writer
}{bytes.NewBuffer(m), &out})
if err == nil {  // ❌ WRONG: Returns error when NO error occurs!
	return fmt.Errorf("ServeAgent err:%v", err)
}
if err != io.EOF {
	return fmt.Errorf("ServeAgent err:%w", err)
}
```

### Root Cause
Inverted condition: treats successful operations (`err == nil`) as errors, causing incorrect error reporting and potential retry loops.

### Fix
Simplified to correct logic:
```go
if err != nil && err != io.EOF {
	return fmt.Errorf("ServeAgent err:%w", err)
}
```

---

## Bug #3: Nil Keyring in Non-Proxy Mode (CRITICAL - Causes Freeze on ssh-add)
**File**: `pkg/sshutil/sshutil.go:213-225` (FIXED)
**Severity**: CRITICAL - Nil pointer dereference

### Problem
```go
func NewKeyRing(s *store.Settings) *KeyRing {
	k := &KeyRing{settings: s}
	if s.ProxyModeOfNamedPipe {
		k.keyring = &namedpipe.NamedPipeClient{}
		return k
	}
	a := agent.NewKeyring()
	if extendedAgent, ok := a.(agent.ExtendedAgent); ok {
		k.keyring = extendedAgent
		return k
	}
	return nil  // ❌ Returns nil when type assertion fails!
}
```

### Root Cause
The standard library's `agent.NewKeyring()` only implements `agent.Agent`, NOT `agent.ExtendedAgent`. When the type assertion fails, the function returns `nil`, leaving `k.keyring` as `nil`. When `ssh-add` queries the socket and calls methods like `List()`, it attempts to call `k.keyring.List()` on a nil pointer, causing a panic or hang.

### Fix
Created an `extendedKeyringWrapper` that wraps the basic `agent.Agent` and implements the missing `ExtendedAgent` methods:

```go
// Wrap the basic Agent with ExtendedAgent methods
k.keyring = &extendedKeyringWrapper{Agent: a}
return k

// extendedKeyringWrapper wraps agent.Agent to implement agent.ExtendedAgent
type extendedKeyringWrapper struct {
	agent.Agent
}

func (w *extendedKeyringWrapper) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	// Fall back to Sign without flags
	return w.Sign(key, data)
}

func (w *extendedKeyringWrapper) Extension(extensionType string, contents []byte) ([]byte, error) {
	return nil, errors.New("extension not supported")
}
```

---

## Why These Cause Freezes

1. **Bug #1 (Infinite Recursion)**: Immediate stack overflow when `Signers()` is called
2. **Bug #2 (Inverted Logic)**: Causes incorrect error handling in Pageant, potentially triggering retry loops
3. **Bug #3 (Nil Keyring)**: When `ssh-add` queries the Unix socket, it calls `agent.ServeAgent()` which invokes `List()` on the KeyRing. With a nil keyring, this causes a panic or hang that freezes the agent

## Testing
To verify the fixes work:
```bash
# Start the agent
./OmniSSHAgent.exe

# Query with ssh-add (should not freeze)
ssh-add -l
ssh-add ~/.ssh/id_rsa
```

## Files Modified
- `pkg/sshutil/sshutil.go` - Fixed Signers() recursion and added extendedKeyringWrapper
- `pkg/pageant/pageant.go` - Fixed inverted error logic