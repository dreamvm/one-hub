import unittest

from load_accounting import assert_load_accounting


class LoadAccountingTests(unittest.TestCase):
    def states(self):
        before = {"quota": 1000000, "used": 140, "requests": 10,
                  "finite_used": 0, "finite_remaining": 1000000,
                  "unlimited_used": 0, "unlimited_remaining": 0,
                  "receipts": 10, "consumed": 10, "receipt_cost": 140,
                  "logs": 10, "log_cost": 140, "channel_cost": 140, "pending": 0}
        after = {"quota": 999552, "used": 588, "requests": 42,
                 "finite_used": 224, "finite_remaining": 999776,
                 "unlimited_used": 0, "unlimited_remaining": 0,
                 "receipts": 42, "consumed": 42, "receipt_cost": 588,
                 "logs": 42, "log_cost": 588, "channel_cost": 588, "pending": 0}
        return before, after

    def test_mixed_finite_unlimited_exact_settlement(self):
        before, after = self.states()
        assert_load_accounting(before, after, 14, 32, 16)

    def test_missing_duplicate_or_misallocated_accounting_is_rejected(self):
        before, after = self.states()
        for field in after:
            for discrepancy in (-1, 1):
                with self.subTest(field=field, discrepancy=discrepancy):
                    changed = dict(after)
                    changed[field] += discrepancy
                    with self.assertRaises(AssertionError):
                        assert_load_accounting(before, changed, 14, 32, 16)

    def test_unresolved_baseline_and_unbounded_wave_are_rejected(self):
        before, after = self.states()
        before["pending"] = 1
        with self.assertRaises(AssertionError):
            assert_load_accounting(before, after, 14, 32, 16)
        before["pending"] = 0
        for cost, requests, finite in ((0, 32, 16), (14, 33, 16), (14, 32, 33)):
            with self.assertRaises(AssertionError):
                assert_load_accounting(before, after, cost, requests, finite)


class LoadMetricTests(unittest.TestCase):
    body = '''# TYPE go_goroutines gauge
go_goroutines 18
process_open_fds 22
process_resident_memory_bytes 6.4e+07
http_requests_total{code="200",method="POST",path="/v1/chat/completions"} 42
http_request_duration_seconds_count{path="/v1/chat/completions",method="POST",code="200"} 42
http_requests_total{code="404",method="GET",path="/api/metrics"} 2
'''

    def test_exact_route_metrics_and_process_gauges(self):
        from load_accounting import read_load_metrics
        values = read_load_metrics(self.body)
        self.assertEqual(values["http_requests_total"], 42)
        self.assertEqual(values["http_request_duration_seconds_count"], 42)
        self.assertEqual(values["process_resident_memory_bytes"], 64000000)

    def test_missing_wrong_route_duplicate_and_invalid_samples(self):
        from load_accounting import read_load_metrics
        for body in (self.body.replace("go_goroutines 18\n", ""),
                     self.body.replace('path="/v1/chat/completions"', 'path="/fixture/other"'),
                     self.body + "go_goroutines 18\n",
                     self.body.replace("go_goroutines 18", "go_goroutines -1"),
                     self.body.replace("go_goroutines 18", "go_goroutines 1e999")):
            with self.subTest(body=body), self.assertRaises(AssertionError):
                read_load_metrics(body)


class LoadWaveCleanupTests(unittest.TestCase):
    def test_normal_wave_completes_all_requests(self):
        from load_accounting import run_load_wave
        self.assertEqual(run_load_wave(lambda index: index), list(range(32)))

    def test_failure_timeout_and_cancellation_stop_pending_work(self):
        from load_accounting import run_load_wave
        from threading import Barrier, Lock
        import time
        for failure in (RuntimeError, TimeoutError, KeyboardInterrupt):
            with self.subTest(failure=failure):
                started = []
                lock = Lock()
                barrier = Barrier(4)
                def operation(index):
                    with lock:
                        started.append(index)
                    barrier.wait(timeout=1)
                    if index == 0:
                        raise failure("synthetic probe stopped")
                    time.sleep(0.02)
                    return index
                with self.assertRaises(failure):
                    run_load_wave(operation)
                self.assertEqual(sorted(started), [0, 1, 2, 3])


if __name__ == "__main__":
    unittest.main()
