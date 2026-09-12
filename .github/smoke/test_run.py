import json
import unittest
from unittest.mock import patch

from run import SIGNATURES, NAMES, check_tools, parse_chat, start_container


class StreamValidationTests(unittest.TestCase):
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
