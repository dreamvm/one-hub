# Graceful shutdown candidate

This is an unpublished candidate based on RC11 commit
`650d474b553f6756f118f0529cd94cb93fa6710d`. It does not change RC10 or the
published RC11 image. No schema migration is added.

SIGTERM and SIGINT close HTTP admission and the listener. Already admitted
handlers keep their original contexts, ResponseWriter interfaces and time to
finish normal/streaming responses and deferred accounting. The process then
stops scheduling, waits scheduled jobs, cancels and joins registered background
writers, replays only determinate quota terminal intent, rejects unresolved
ledger or in-memory intent, and closes SQL before logging `shutdown_complete`.
This sequence assumes the deployment's database is not also being written by
another application instance or an unregistered external writer.

`SHUTDOWN_TIMEOUT` is a Go duration, default `30s`, strictly positive and at most
`2m`. All shutdown phases share that single deadline. The local controller must
budget it within its existing stop/window limits; this setting does not extend
the independent initial 300-second/subsequent 3600-second evidence ages or the
180-second restore limit. Main waits for the sequence. A second TERM/INT does
not bypass the deadline. SIGKILL is uncatchable; SIGQUIT retains Go's diagnostic
failure behavior. Neither has a successful shutdown marker.

Any incomplete phase, join error, unresolved/unknown accounting or close error
returns exit 1 without a success marker. No later phase runs after a failed
phase; in particular SQL is not closed underneath known unfinished handlers or
writers. A timed-out callback may still be running until the process exits: a
deadline is not evidence of goroutine termination. No pending receipt is
automatically refunded. Preserve logs/inspect/ledger before controller cleanup.

The initial supported success profile excludes batch buffered writes, MCP and
Telegram. `BATCH_UPDATE_ENABLED=true`, MCP enabled or a configured Telegram bot
key make the worker phase fail closed. Those existing paths lack a reviewed
complete flush/join/error contract; this candidate neither silently disables
them nor claims to support their successful shutdown. Add those contracts in a
separately scoped change if the deployment requires them. Existing durable
provider task rows can remain unfinished while their local pollers are joined;
they resume through the existing startup path and are not refunded on shutdown.

A successful application exit is evidence for this owned lifecycle, not a
backup approval or an external ingress fence. Maintenance must still bind the
exact new program, signal/events/inspect, log continuity, admitted-work coverage,
database quietness and all existing backup/resource/restore checks. Never treat
exit 2 or `--init` as this repair. Fixed RC10 cannot provide this candidate's
in-memory/background guarantees; its first-switch proof remains blocked when
authoritative evidence is unavailable.
