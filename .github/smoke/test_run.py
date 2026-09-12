import json
from pathlib import Path
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch

from run import SIGNATURES, NAMES, MySQLRedis, check_tools, parse_chat, start_container, validate_backend


class StreamValidationTests(unittest.TestCase):
    def test_mysql_redis_rejects_mutable_or_missing_images(self):
        pinned = "sha256:" + "a" * 64
        for invalid in (None, "mysql:8.2.0", "redis:latest", "sha256:123", "production-mysql"):
            for mysql, redis in ((invalid, pinned), (pinned, invalid)):
                with self.assertRaises(AssertionError):
                    validate_backend(SimpleNamespace(backend="mysql-redis", mysql_image=mysql, redis_image=redis))
        validate_backend(SimpleNamespace(backend="mysql-redis", mysql_image=pinned, redis_image=pinned))
        validate_backend(SimpleNamespace(backend="sqlite"))

    def test_mysql_redis_uses_only_new_volume_and_internal_aliases(self):
        pinned = "sha256:" + "a" * 64
        args = SimpleNamespace(mysql_image=pinned, redis_image=pinned)
        created = []
        with tempfile.TemporaryDirectory() as tmp, patch("run.command"), \
                patch("run.start_container") as start, patch.object(MySQLRedis, "wait_ready"):
            backend = MySQLRedis(args, "onehub-smoke-test", Path(tmp),
                                 ["--network", "onehub-smoke-test-net", "--cap-drop", "ALL"], created)
            self.assertEqual(created, [("volume", "onehub-smoke-test-mysql-data")])
            self.assertEqual(start.call_count, 2)
            for call in start.call_args_list:
                name, options, tracked = call.args
                self.assertTrue(name.startswith("onehub-smoke-test-"))
                self.assertIs(tracked, created)
                self.assertNotIn("-p", options)
                self.assertNotIn("--publish", options)
                self.assertIn("--memory", options)
                self.assertIn("--cpus", options)
                self.assertIn("999:999", options)
            env = backend.environment()
            self.assertTrue(any("@tcp(database:3306)/onehub_smoke?" in value for value in env))
            self.assertTrue(any("@cache:6379/0" in value for value in env))
            self.assertIn("SYNC_FREQUENCY=600", env)

    def test_failed_container_start_remains_tracked_for_cleanup(self):
        created = []
        with patch("run.command", side_effect=[None, RuntimeError("OCI start failed")]):
            with self.assertRaises(RuntimeError):
                start_container("onehub-smoke-fixture", ["fixture-image"], created)
        self.assertEqual(created, [("container", "onehub-smoke-fixture")])

    def stream(self, chunks, done=True):
        return "".join("data: " + json.dumps(c) + "\n\n" for c in chunks) + ("data: [DONE]\n\n" if done else "")

    def parse(self, chunks, done=True):
        return parse_chat(200, {"Content-Type": "text/event-stream"}, self.stream(chunks, done), True)

    def test_reassembles_chinese_text(self):
        message, usage = self.parse([
            {"choices": [{"delta": {"content": "中文"}}]},
            {"choices": [{"delta": {"content": "成功"}, "finish_reason": "stop"}], "usage": {"total_tokens": 10}},
        ])
        self.assertEqual(message["content"], "中文成功")
        self.assertEqual(usage["total_tokens"], 10)

    def test_rejects_truncated_stream_and_embedded_errors(self):
        with self.assertRaises(AssertionError):
            self.parse([{"choices": [{"delta": {}, "finish_reason": "stop"}]}], done=False)
        with self.assertRaises(AssertionError):
            self.parse([{"choices": [{"delta": {}}]}])
        with self.assertRaises(AssertionError):
            self.parse([{"error": {"message": "interrupted"}}])

    def test_two_tool_headers_and_fragmented_arguments(self):
        chunks = []
        for i, name in enumerate(NAMES):
            for part in [
                {"index": i, "id": f"tool-{i}", "type": "function", "function": {"name": name, "arguments": ""}, "extra_content": {"google": {"thought_signature": SIGNATURES[i]}}},
                {"index": i, "function": {"arguments": '{"file":"中文'}},
                {"index": i, "function": {"arguments": '测试.docx"}'}},
            ]:
                chunks.append({"choices": [{"delta": {"tool_calls": [part]}}]})
        chunks.append({"choices": [{"delta": {}, "finish_reason": "tool_calls"}]})
        message, _ = self.parse(chunks)
        check_tools(message)
        message["tool_calls"][1]["id"] = "tool-0"
        with self.assertRaises(AssertionError):
            check_tools(message)
        with self.assertRaises(AssertionError):
            self.parse([chunks[0]] + chunks)


if __name__ == "__main__":
    unittest.main()
