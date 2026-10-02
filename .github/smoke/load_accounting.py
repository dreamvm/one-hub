"""Exact deltas for bounded synthetic concurrent relay acceptance."""


def assert_load_accounting(before, after, cost, requests, finite_requests):
    if cost <= 0 or not 0 < requests <= 32 or not 0 <= finite_requests <= requests:
        raise AssertionError("invalid synthetic wave size or price")
    expected = {
        "quota": -cost * requests,
        "used": cost * requests,
        "requests": requests,
        "finite_used": cost * finite_requests,
        "finite_remaining": -cost * finite_requests,
        "unlimited_used": 0,
        "unlimited_remaining": 0,
        "receipts": requests,
        "consumed": requests,
        "receipt_cost": cost * requests,
        "logs": requests,
        "log_cost": cost * requests,
        "channel_cost": cost * requests,
    }
    if before["pending"] != 0 or after["pending"] != 0:
        raise AssertionError("concurrent wave left unresolved reservations")
    for key, delta in expected.items():
        if after[key] - before[key] != delta:
            raise AssertionError(f"concurrent wave accounting mismatch: {key}")


def read_load_metrics(body):
    """Read only the fixture's known counters and process gauges."""
    import math
    import re

    result = {}
    for line in body.splitlines():
        match = re.fullmatch(r'([a-zA-Z_:][a-zA-Z0-9_:]*)(\{[^\n]*\})? ([0-9.eE+\-]+)', line)
        if not match:
            continue
        name, labels, raw = match.groups()
        if name in ("go_goroutines", "process_open_fds", "process_resident_memory_bytes") and labels is None:
            key = name
        elif name in ("http_requests_total", "http_request_duration_seconds_count") and labels and all(
                label in labels[1:-1].split(",") for label in ('method="POST"', 'path="/v1/chat/completions"', 'code="200"')):
            key = name
        else:
            continue
        value = float(raw)
        if key in result or not math.isfinite(value) or value < 0:
            raise AssertionError("invalid or duplicate fixture metric")
        result[key] = value
    if len(result) != 5:
        raise AssertionError("required fixture metrics missing")
    return result


def run_load_wave(operation):
    """Stop queued work on failure; callers bound each running probe to 25 seconds."""
    from concurrent.futures import CancelledError, ThreadPoolExecutor
    from threading import Event
    import time

    stopped = Event()
    deadline = time.monotonic() + 45

    def guarded(index):
        if stopped.is_set():
            raise CancelledError("synthetic load wave stopped")
        try:
            return operation(index)
        except BaseException:
            stopped.set()
            raise

    executor = ThreadPoolExecutor(max_workers=4)
    futures = []
    try:
        futures = [executor.submit(guarded, index) for index in range(32)]
        return [future.result(timeout=max(0, deadline - time.monotonic())) for future in futures]
    finally:
        stopped.set()
        for future in futures:
            future.cancel()
        executor.shutdown(wait=True, cancel_futures=True)
