import copy
import json
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from run import OwnedNetworkPartition


class NetworkPartitionTests(unittest.TestCase):
    network = "onehub-smoke-123456abcdef-net"
    target = "onehub-smoke-123456abcdef-postgres"

    def fixture(self, fail_disconnect=False):
        state = {"attached": True, "process": "true 42", "connections": [], "address": "172.20.0.4"}
        attachment = {"IPAddress": state["address"], "Aliases": ["database", self.target]}

        def command(*args):
            if args[:3] == ("docker", "network", "inspect"):
                return SimpleNamespace(stdout="true\n")
            if args[:2] == ("docker", "inspect"):
                if args[3] == "{{.State.Running}} {{.State.Pid}}":
                    return SimpleNamespace(stdout=state["process"])
                return SimpleNamespace(stdout=json.dumps({self.network: attachment} if state["attached"] else {}))
            if args[:3] == ("docker", "network", "disconnect"):
                self.assertEqual(args[3:], (self.network, self.target))
                state["attached"] = False
                if fail_disconnect:
                    raise RuntimeError("fixture command failed after removing attachment")
                return SimpleNamespace(stdout="")
            if args[:3] == ("docker", "network", "connect"):
                self.assertEqual(args[-2:], (self.network, self.target))
                self.assertIn("--ip", args)
                self.assertEqual(args[args.index("--ip") + 1], state["address"])
                for alias in attachment["Aliases"]:
                    self.assertIn(alias, args)
                state["connections"].append(args)
                state["attached"] = True
                return SimpleNamespace(stdout="")
            raise AssertionError("unexpected fixture operation")

        return state, command

    def test_exception_restores_identity_and_repeat_restore_is_noop(self):
        for ambiguous in (False, True):
            with self.subTest(ambiguous_disconnect=ambiguous):
                state, command = self.fixture(fail_disconnect=ambiguous)
                with patch("run.command", side_effect=command):
                    partition = OwnedNetworkPartition(self.network, self.target)
                    with self.assertRaises(RuntimeError):
                        try:
                            partition.disconnect()
                            raise RuntimeError("fixture request failed")
                        finally:
                            partition.restore()
                    partition.restore()
                self.assertTrue(state["attached"])
                self.assertEqual(len(state["connections"]), 1)

    def test_restarted_process_is_not_accepted_as_partition_recovery(self):
        state, command = self.fixture()
        with patch("run.command", side_effect=command):
            partition = OwnedNetworkPartition(self.network, self.target)
            partition.disconnect()
            state["process"] = "true 43"
            with self.assertRaisesRegex(AssertionError, "restarted"):
                partition.restore()
        self.assertTrue(state["attached"], "restore the attachment even when process validation fails")

    def test_foreign_target_is_refused_before_docker_access(self):
        for network, target in (("production", self.target), (self.network, "production-database"),
                                (self.network, "onehub-smoke-aaaaaaaaaaaa-postgres"),
                                (self.network, "onehub-smoke-123456abcdef-gateway")):
            with self.subTest(target=target), patch("run.command") as command:
                with self.assertRaises(AssertionError):
                    OwnedNetworkPartition(network, target)
                command.assert_not_called()

    def test_restoration_requires_original_ip_and_aliases(self):
        for changed in ({"IPAddress": "172.20.0.8", "Aliases": ["database", self.target]},
                        {"IPAddress": "172.20.0.4", "Aliases": []}):
            state, command = self.fixture()
            with patch("run.command", side_effect=command):
                partition = OwnedNetworkPartition(self.network, self.target)
                partition.disconnect()
                initial = partition.attachments
                calls = 0

                def attachments():
                    nonlocal calls
                    calls += 1
                    return initial() if calls == 1 else {self.network: copy.deepcopy(changed)}

                with patch.object(partition, "attachments", side_effect=attachments):
                    with self.assertRaisesRegex(AssertionError, "identity"):
                        partition.restore()
                # A failed validation leaves restoration retryable; no process restart workaround.
                partition.restore()
                self.assertFalse(partition.restore_needed)
                self.assertTrue(state["attached"])


if __name__ == "__main__":
    unittest.main()
