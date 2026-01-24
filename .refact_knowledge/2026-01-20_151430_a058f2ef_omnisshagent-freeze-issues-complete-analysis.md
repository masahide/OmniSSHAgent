---
id: "8b634699-9518-4664-9563-10a044e3e208"
title: "OmniSSHAgent Freeze Issues: 6 Bugs Fixed"
kind: code
created: 2026-01-20
updated: 2026-01-20
review_after: 2026-04-20
status: deprecated
tags: ["bugfix", "deadlock", "extended-runtime", "file-descriptor-leak", "freeze", "goroutine", "goroutine-leak", "leak", "recursion", "resource-leak", "ssh-agent", "sshagent", "timeout"]
filenames: ["app.go", "pkg/cygwinsocket/cygwinsocket.go", "pkg/namedpipe/namedpipe.go", "pkg/pageant/pageant.go", "pkg/sshutil/sshutil.go", "pkg/unix/unix.go"]
links: ["b552d667-1677-48dd-b09c-9ae19a4e4101"]
superseded_by: "bba34568-0de1-4384-90c5-d7594e21e99c"
deprecated_at: 2026-01-20
---


> ⚠️ **DEPRECATED**: Covers OmniSSHAgent freeze issues including deadlock and goroutine leaks in overlapping files (app.go). New document provides more specific, updated root cause analysis of circular callback deadlock (a subset of the 6 bugs fixed), making the older general doc outdated.

# OmniSSHAgent Freeze Issues - Complete Analysis and Fixes

## Summary
Found and fixed **6 critical bugs** causing the SSH agent to freeze at runtime:
- 3 immediate freeze bugs (infinite recursion, inverted logic, nil keyring)
- 3 resource leak bugs (missing connection close, untracked goroutines, no read timeout)

---

## Immediate Freeze Bugs (Fixed Earlier)

### Bug #1: Infinite Recursion in KeyRing.Signers()
**File**: `pkg/sshutil/sshutil.go:385`
**Fix**: Changed `return k.Signers()` → `return k.keyring.Signers()`

### Bug #2: Inverted Error Logic in Pageant
**File**: `pkg/pageant/pageant.go:142-146`
**Fix**: Corrected `if err == nil` → `if err != nil && err != io.EOF`

### Bug #3: Nil Keyring in Non-Proxy Mode
**File**: `pkg/sshutil/sshutil.go:213-225`
**Fix**: Created `extendedKeyringWrapper` to wrap `agent.Agent` and implement missing `ExtendedAgent` methods

---

## Resource Leak Bugs (Cause Freeze After Extended Runtime)

### Bug #4: Missing Connection Close in Cygwin Socket Handler
**File**: `pkg/cygwinsocket/cygwinsocket.go:92`
**Severity**: HIGH - File descriptor leak
**Problem**: `handle()` method never closes the connection, causing FD exhaustion over time
**Fix**: Added `defer conn.Close()` at the start of the function

### Bug #5: Untracked Agent Goroutines
**File**: `app.go:122, 129, 133, 138`
**Severity**: CRITICAL - Goroutine leak
**Problem**: Agent goroutines spawned with `go` but NOT added to WaitGroup, causing:
- Goroutines accumulate indefinitely
- Each connection spawns a handler goroutine that blocks on `agent.ServeAgent()`
- Stale/hung connections cause handler goroutines to block forever
- Over time: hundreds of blocked goroutines → memory exhaustion → system freeze

**Fix**: 
1. Added `a.wg.Add(1)` before each agent goroutine spawn
2. Wrapped each agent spawn with `defer a.wg.Done()`
3. Added proper error logging for agent failures
4. Added `cancelAgents` context to App struct for graceful shutdown

### Bug #6: No Read Timeout on Agent Connections
**Files**: 
- `pkg/unix/unix.go:60-69`
- `pkg/namedpipe/namedpipe.go:49-58`
- `pkg/cygwinsocket/cygwinsocket.go:92-121`

**Severity**: CRITICAL - Indefinite blocking
**Problem**: `agent.ServeAgent()` blocks indefinitely waiting for client data. If a client:
- Connects but never sends data
- Sends partial data and hangs
- Disconnects abruptly without proper close

The handler goroutine blocks forever, accumulating over time.

**Fix**: Added 5-minute read deadline to all connection handlers:
```go
conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
```

This ensures that stale connections are automatically closed after 5 minutes of inactivity, freeing the goroutine.

---

## Why These Cause Freezes

**Immediate Freeze (Bugs #1-3)**:
- Stack overflow or nil pointer dereference when agent methods are called

**Delayed Freeze (Bugs #4-6)**:
1. Client connects to Unix socket
2. `ssh-add` or other SSH tool queries the agent
3. Handler goroutine calls `agent.ServeAgent(conn)`
4. If connection stalls or client hangs, `ServeAgent()` blocks indefinitely
5. Handler goroutine never exits, stays blocked
6. Over time, hundreds of connections accumulate
7. Each blocked goroutine consumes memory and resources
8. File descriptors get exhausted (Bug #4)
9. System runs out of resources → freeze

---

## Files Modified

1. **pkg/sshutil/sshutil.go**
   - Fixed `Signers()` infinite recursion
   - Added `extendedKeyringWrapper` for nil keyring fix

2. **pkg/pageant/pageant.go**
   - Fixed inverted error logic

3. **pkg/cygwinsocket/cygwinsocket.go**
   - Added `defer conn.Close()`
   - Added read timeout

4. **pkg/unix/unix.go**
   - Added read timeout

5. **pkg/namedpipe/namedpipe.go**
   - Added read timeout

6. **app.go**
   - Added `cancelAgents` and `agentContexts` to App struct
   - Wrapped all agent goroutines with WaitGroup tracking
   - Added proper shutdown with context cancellation
   - Added error logging for agent failures

---

## Testing

```bash
# Start the agent
./OmniSSHAgent.exe

# Query with ssh-add (should not freeze)
ssh-add -l
ssh-add ~/.ssh/id_rsa

# Leave running for extended time - should not accumulate goroutines
# Monitor with: go tool pprof http://localhost:6060/debug/pprof/goroutine
```

---

## Root Cause Summary

The agent froze after some time due to **goroutine accumulation**:
- Agent handlers were not tracked by WaitGroup
- `agent.ServeAgent()` had no timeout, blocking indefinitely on stale connections
- Each stale connection leaked a goroutine
- Over time, hundreds of goroutines accumulated
- File descriptors were exhausted (Cygwin missing close)
- System resources depleted → freeze

All issues are now fixed with proper goroutine tracking, timeouts, and resource cleanup.