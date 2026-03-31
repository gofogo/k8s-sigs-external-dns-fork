# controller.go — Review Findings

## Issues

### 1. Dead field: `hostname` (Low)

`hostname` is read via `os.Hostname()` in `NewEventController` (line 59) and stored on the struct (line 51), but is never referenced anywhere — not in `processNextWorkItem`, `emit`, or `run`. Original intent was likely to populate `event.ReportingInstance`, but that is already done in `types.go` as `controllerName + "/source/" + e.ref.source`.

**Fix:** Remove the `hostname` field from the struct and the `os.Hostname()` call from `NewEventController`.

---

### 2. Misleading error return on `NewEventController` (Low)

```go
func NewEventController(client v1.EventsV1Interface, cfg *Config) (*Controller, error)
```

The function never returns a non-nil error. `os.Hostname()` failure is silently discarded with `_`. Callers in `controller/execute.go` are forced to handle a phantom error. Either remove the error return or surface the `os.Hostname` failure (once `hostname` is removed, remove the error return entirely).

**Fix:** Change signature to `func NewEventController(...) *Controller`. Update `controller/execute.go` and `controller_test.go`.

---

### 3. Misleading constant: `maxTriesPerEvent` (Low)

The constant is named "max tries" but semantically means "max retries". `NumRequeues` counts `AddRateLimited` calls (retries), not total attempts. With `maxTriesPerEvent = 3`, the item is attempted **4 times** (1 initial + 3 retries). The test acknowledges this by looping `maxTriesPerEvent + 1` times.

**Fix:** Rename to `maxRetriesPerEvent`.

---

### 4. Redundant `ReportingController` assignment in `emit` (Low)

```go
// emit() — controller.go:146
event.ReportingController = controllerName
```

`event()` in `types.go` already sets `event.ReportingController = controllerName` before the event reaches `emit`. The assignment in `emit` is a no-op and creates a silent second source of truth.

**Fix:** Remove the assignment from `emit`.

---

### 5. Dead reactor in `TestController_ProcessNextWorkItem_RequeuesOnError` (Medium / Test bug)

Two reactors are registered with `PrependReactor` on the same client (lines 320–336). Because `PrependReactor` is LIFO, the **second** registered reactor fires first and always returns `(true, ...)`, short-circuiting the chain. The first reactor (lines 320–323) is dead code and never executes.

**Fix:** Remove the first (dead) reactor; keep only the second one (which owns the `eventCreated` channel and the correct retry logic).

---

### 6. Soft TOCTOU on queue length in `Add` (Info / Benign)

```go
if ec.queue.Len() >= ec.maxQueuedEvents {
```

`Add` is called from the reconciliation goroutine while the worker drains the queue concurrently. The length check and the subsequent `queue.Add` in `emit` are not atomic, so slightly more than `maxQueuedEvents` items can be enqueued under burst load. Acceptable for best-effort telemetry, but `maxQueuedEvents` is a soft limit, not a hard one.

**No fix required** — worth a comment acknowledging it.

---

### 7. Graceful-shutdown retry extension (Info)

During shutdown, context cancellation causes in-flight `Create` calls to return an error. If `NumRequeues < maxRetriesPerEvent`, the item is requeued with `AddRateLimited`. `ShutDownWithDrain` waits for all in-flight items to be processed, so shutdown can take up to `maxRetriesPerEvent × rate-limiter-delay` longer than expected. Acceptable for best-effort telemetry but worth a comment.

**No fix required** — document in code.

---

## Summary

| # | Severity | Action |
|---|---|---|
| 1 | Low | Remove `hostname` field and `os.Hostname()` call |
| 2 | Low | Remove error return from `NewEventController` |
| 3 | Low | Rename `maxTriesPerEvent` → `maxRetriesPerEvent` |
| 4 | Low | Remove redundant `ReportingController` assignment from `emit` |
| 5 | Medium | Remove dead reactor from `TestController_ProcessNextWorkItem_RequeuesOnError` |
| 6 | Info | Add comment noting soft queue-length limit |
| 7 | Info | Add comment on shutdown retry extension |
