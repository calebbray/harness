# Stage 8 — Async Task Queue (Epic)

**Goal:** turn the harness from a single synchronous REPL conversation into
a personal-assistant-style tool that can run background jobs (e.g. "pull
sprint status from three Jira boards") without blocking the REPL, track
their status, and eventually run them on a schedule.

**Foundational constraint carried in from Stage 6/7:** `Client` and
`Agent.Step()` stay exactly as synchronous as they are today. All
concurrency lives in a new layer that *wraps* `Agent` — nothing in `Agent`
or `Client` should need to change to support any of this.

**Key design decision made up front:** each job gets its own `Agent`
instance (same shared `Client` and `Registry`, fresh `history`). Jobs do
not share conversation history with the REPL or with each other. This is
the load-bearing decision for the whole epic — get it right early and the
queue/pool/status work stays simple; get it wrong and later stages fight
subtle data races instead of building features.

---

## Story 8.0 — Define what a `Job` is (no concurrency)

Build a plain `Job` struct: ID, instruction text, status
(`pending`/`running`/`done`/`failed`), and eventually a result. No
channels, no goroutines yet.

**Done when:** a `Job` can be constructed by hand in a test/scratch call,
with no runtime behavior attached yet.

## Story 8.1 — In-memory queue, fully synchronous

A `Queue` type (slice or channel-backed) with `Enqueue`/`Dequeue`. From the
REPL, manually enqueue a job, then manually dequeue and run it (call
`agent.Step()` on a throwaway `Agent`) in the same blocking call, to prove
the queue mechanics and the "job → fresh agent → completion" path before
any concurrency is introduced.

**Done when:** two jobs can be enqueued, then dequeued and run one after
another, each completing independently without interfering with the other.

## Story 8.2 — One worker goroutine

Turn the manual dequeue-and-run from 8.1 into a goroutine that loops: pull
a job off the queue (buffered `chan Job` is a natural fit), run it, update
its status, repeat. The REPL's main loop enqueues and returns immediately —
first point where the REPL is genuinely non-blocking.

**Done when:** a job with an artificially slow command (e.g. `sleep 10` via
`bash`) is enqueued, and the REPL stays responsive to new input while that
job is still running in the background.

## Story 8.3 — Job status tracking + `/tasks` REPL command

A shared store for job status (e.g. `map[string]*Job` guarded by a mutex —
first point requiring explicit synchronization, since the worker goroutine
writes status while the REPL goroutine reads it). Wire up a `/tasks`
command in the REPL to list jobs and their current status/result.

**Done when:** a job can be enqueued, immediately checked via `/tasks`
showing `running`, and later checked again showing `done` with a result.

## Story 8.4 — Worker pool (fan-out)

Generalize the single goroutine from 8.2 into N goroutines all ranging over
the same job channel — multiple goroutines reading one channel naturally
load-balances work with no extra coordination code needed. Fixed pool size
(e.g. 3) is fine to start; no dynamic scaling.

**Done when:** three `sleep`-based jobs enqueued at once genuinely run
concurrently — their completion times overlap rather than stack up
serially.

## Story 8.5 — Confirmation semantics for background jobs

Real design gap: the existing `Confirmer` prompts a human synchronously at
a terminal, which doesn't exist for a background job. Default posture:
**auto-decline any confirmation-requiring tool when running inside a
background job.** Revisit only if this turns out to block something
genuinely needed — don't build a queued-approval mechanism preemptively.

**Done when:** a background job that calls a confirmation-requiring tool
(e.g. `write_file`) completes with that tool call recorded as declined,
rather than hanging or silently running unattended.

## Story 8.6 — Recurring / scheduled jobs

Deliberately last, and genuinely separable from everything above: a
scheduler (ticker or cron-like check) that calls the same `Enqueue` path
used by the REPL, on a timer, with a preconfigured job description. Once
8.0–8.5 work, this stage should be close to purely mechanical.

**Done when:** a scheduled job fires automatically at the configured time
and produces the same tracked, queryable result as a manually-enqueued job.

---

## Notes carried forward

- `slog`'s stock handlers are internally mutex-protected, so multiple
  worker goroutines logging through the shared fanout `*slog.Logger`
  should interleave safely — worth confirming under real concurrency in
  8.4, not just assuming.
- Definitions of done stay concrete and testable at each story, same
  discipline as Stages 0–7 — no story is "done" on vibes or on "it
  compiles."
