from pathlib import Path
import subprocess
import sys
import unittest


class ComposeCleanupTests(unittest.TestCase):
    def exercise(self, mode):
        script = r'''
import json, os, runpy, signal, subprocess, sys
mode, path = sys.argv[1:]
def fake(command, **kwargs):
    action = next(value for value in ("up", "exec", "down") if value in command)
    print("INVOKE " + action, flush=True)
    if action == "up" and mode == "signal":
        os.kill(os.getpid(), signal.SIGTERM)
    status = 1 if action == "up" and mode == "failure" else 0
    output = json.dumps({"success": True, "data": {"version": "fixture"}}) if action == "exec" else ""
    return subprocess.CompletedProcess(command, status, output, "")
subprocess.run = fake
sys.argv = [path, "--compose", "fixture-compose", "--image", "sha256:" + "a" * 64, "--version", "fixture"]
runpy.run_path(path, run_name="__main__")
'''
        return subprocess.run([sys.executable, "-c", script, mode, str(Path(__file__).with_name("compose.py"))],
                              text=True, capture_output=True, timeout=15)

    def test_success_cleans_all_four_modes(self):
        result = self.exercise("success")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.count("INVOKE up"), 4)
        self.assertEqual(result.stdout.count("INVOKE down"), 4)

    def test_up_failure_still_cleans(self):
        result = self.exercise("failure")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout.count("INVOKE down"), 1)

    def test_sigterm_still_cleans(self):
        result = self.exercise("signal")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout.count("INVOKE down"), 1)


if __name__ == "__main__":
    unittest.main()
