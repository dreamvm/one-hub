import json
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from run import SQLiteFixture


class SQLiteFixtureTests(unittest.TestCase):
    volume = "onehub-smoke-123456abcdef-data"
    mock = "onehub-smoke-123456abcdef-mock"

    def mount(self, writable=False):
        return {"Type": "volume", "Name": self.volume, "Destination": "/fixture-db", "RW": writable}

    def test_owned_readonly_mount_and_fixed_reader_command(self):
        replies = [SimpleNamespace(stdout=json.dumps([self.mount()])), SimpleNamespace(stdout='{"values":[3,4]}')]
        with patch("run.command", side_effect=replies) as command:
            fixture = SQLiteFixture(self.mock, self.volume)
            self.assertEqual(fixture.read("tokens", 2), [3, 4])
            call = command.call_args
            self.assertEqual(call.args, ("docker", "exec", "-i", self.mock, "/fixture/mock", "sqlite-fixture"))
            self.assertEqual(json.loads(call.kwargs["input_text"]), {"Action": "tokens", "UserID": 2, "FiniteID": 0, "UnlimitedID": 0})

    def test_foreign_target_refused_before_access(self):
        with patch("run.command") as command:
            for mock, volume in (("production", self.volume), (self.mock, "production")):
                with self.assertRaises(AssertionError):
                    SQLiteFixture(mock, volume)
            command.assert_not_called()

    def test_writable_or_wrong_volume_refused(self):
        for mounts in ([self.mount(True)], [], [dict(self.mount(), Name="foreign-fixture")]):
            with patch("run.command", return_value=SimpleNamespace(stdout=json.dumps(mounts))):
                with self.assertRaises(AssertionError):
                    SQLiteFixture(self.mock, self.volume)

    def test_incomplete_or_invalid_read_is_not_zero_accounting(self):
        for raw in ('{"error":"fixture accounting read failed"}', '{"values":[0]}', '{"values":[true,4]}'):
            replies = [SimpleNamespace(stdout=json.dumps([self.mount()])), SimpleNamespace(stdout=raw)]
            with patch("run.command", side_effect=replies):
                fixture = SQLiteFixture(self.mock, self.volume)
                with self.assertRaises(AssertionError):
                    fixture.read("tokens", 2)
