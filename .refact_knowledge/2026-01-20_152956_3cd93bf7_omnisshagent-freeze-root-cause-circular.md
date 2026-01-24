---
id: "bba34568-0de1-4384-90c5-d7594e21e99c"
title: "OmniSSHAgent Freeze: Circular Callback Deadlock Fix"
kind: code
created: 2026-01-20
updated: 2026-01-20
review_after: 2026-04-20
status: active
tags: ["callback", "circular-callback", "critical-bug", "deadlock", "gc-freeze", "go", "goroutine", "goroutine-leak", "ssh-agent", "synchronous-callback"]
filenames: ["app.go", "pkg/sshutil/sshutil.go"]
links: ["8b634699-9518-4664-9563-10a044e3e208"]
---

# OmniSSHAgent Freeze - Root Cause: Circular Callback Deadlock

## Critical Issue Found
**Symptom**: Agent freezes after some time with 122+ goroutines stuck in `gcBgMarkWorker` (GC deadlock)

**Root Cause**: **Circular synchronous callback deadlock** in the notice mechanism

## The Deadlock Chain

```
1. SSH client connects → agent.ServeAgent() called
2. ServeAgent() calls KeyRing.List()
3. KeyRing.List() calls notice("List", nil) [SYNCHRONOUS]
4. notice() calls App.notice() callback [SYNCHRONOUS]
5. App.notice() calls runtime.EventsEmit("LoadKeysEvent") [SYNCHRONOUS]
6. Frontend receives event → calls App.KeyList()
7. App.KeyList() calls a.keyRing.KeyList()
8. KeyList() calls List() → calls notice() again
9. DEADLOCK: notice() is still executing from step 3!
```

## Why This Causes GC Freeze

- Each SSH connection spawns a handler goroutine
- Handler calls `ServeAgent()` which blocks in the circular callback
- Goroutine can't proceed, can't exit
- 122+ goroutines accumulate in blocked state
- GC tries to run but all goroutines are blocked
- GC itself blocks → **system freeze**

## The Fix

**File**: `pkg/sshutil/sshutil.go:353-360`

**Before** (WRONG - synchronous, causes deadlock):
```go
func (k *KeyRing) notice(action string, data interface{}) {
	if k.NotifyCallback == nil {
		return
	}
	k.NotifyCallback(action, data)  // ❌ BLOCKS HERE
}
```

**After** (CORRECT - asynchronous, prevents deadlock):
```go
func (k *KeyRing) notice(action string, data interface{}) {
	if k.NotifyCallback == nil {
		return
	}
	// Call callback asynchronously to prevent deadlocks
	go k.NotifyCallback(action, data)  // ✅ Non-blocking
}
```

## Why This Works

- Callback now runs in a separate goroutine
- `notice()` returns immediately
- `ServeAgent()` can continue processing
- No circular blocking
- GC can proceed normally

## Call Stack Analysis

The 122 goroutines were stuck at:
```
runtime.gopark (proc.go:425)
runtime.gcBgMarkWorker (mgc.go:1412)
runtime.gcBgMarkStartWorkers.gowrap1 (mgc.go:1328)
```

This indicates GC was blocked waiting for goroutines to reach a safe point, but they were all stuck in the circular callback deadlock.

## Files Modified

- `pkg/sshutil/sshutil.go` - Made `notice()` callback asynchronous

## Testing

```bash
# Start the agent
./OmniSSHAgent.exe

# Query repeatedly with ssh-add (should not freeze)
for i in {1..100}; do ssh-add -l; done

# Monitor goroutines - should stay low
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

## Summary

The agent froze due to a **synchronous callback being called from within an agent handler**, creating a circular dependency:
- Handler → notice() → EventsEmit → Frontend → KeyList() → List() → notice() [BLOCKED]

Making the callback asynchronous breaks the circular chain and allows handlers to complete normally.