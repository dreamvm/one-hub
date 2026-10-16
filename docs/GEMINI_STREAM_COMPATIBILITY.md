# Gemini streaming compatibility

Native Gemini conversion for `/v1/chat/completions` rejects empty `STOP`, malformed
function completions, and incomplete responses instead of emitting a successful
empty completion. Valid text, images, function calls, code execution output,
content filtering, and terminal chunks following earlier content remain supported.
An error after SSE has started is reported through the existing stream error
format without a success `[DONE]` marker.

## Optional preflight

`GEMINI_EMPTY_RESPONSE_RETRY_CHANNELS` is a comma-separated list of explicitly
selected channel IDs, for example `1,2`. It is disabled by default. Selected native
channels pre-read at most 1 MiB of SSE data before starting the shared stream reader.
The request has one total 180-second deadline, including any retry and subsequent
streaming; shorter existing request/client limits still apply. Client cancellation
closes the selected upstream request, including after preflight has committed.

At most one additional request is allowed for an empty `STOP` or empty
`MALFORMED_FUNCTION_CALL`, before any upstream content has been committed downstream.
The serialized request body is identical. Retry is disabled for built-in tools,
multiple requested candidates, non-replayable bodies, and tokens with heartbeat
enabled. Explicit function declarations can suppress channel code-execution
injection, as in the existing conversion. Filters, invalid JSON, transport errors,
token limits, and already-deliverable content never trigger this retry. At the
buffer cap, preflight commits and replays its exact prefix once rather than buffering
or retrying further. Generic relay retries cannot add attempts after a preflight error.

This setting does not change the OpenAI-compatible provider path, native Gemini
passthrough endpoints, or non-streaming conversion. Unselected native converted
streams receive completion validation but do not gain preflight or retries.

## Accounting and validation

Each attempt's native usage is cumulative: keep its last snapshot and add the
discarded attempt once to each final-attempt snapshot. Reject negative/overflowing
counts. Read trailing usage-only events after an empty terminal event within the
same preflight bounds. Preserve the existing text estimate when the final attempt
has no completion usage, even if a discarded attempt reported thought tokens.
There is still one client request, one reservation and one terminal settlement.
Preflight failures use the existing refund path; committed-stream errors preserve
the current settlement policy and report failure on the wire. No database schema,
global retry policy or other provider's accounting is changed.

Offline regression tests cover classification, replay limits, cancellation, body
ownership, snapshot replacement, and limited/unlimited-token SQLite settlement.
Run `go test -race -count=1 ./providers/gemini ./relay ./model ./types ./common/requester`.
Disabling the environment option removes preflight/retry; completion validation
remains. Reverting the compatibility commit removes both changes. Deployment and
new image publication require their separate release approvals.
