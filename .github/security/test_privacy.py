import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
CHECK = ROOT / ".github/security/check_secrets.py"
SCANNER = os.environ.get("GITLEAKS_BIN")


class StagedSecretTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="onehub-privacy-test-")
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name) / "repo with spaces"
        self.repo.mkdir()
        self.env = dict(os.environ, GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull)
        for key in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR",
                    "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME",
                    "GIT_COMMITTER_EMAIL"):
            self.env.pop(key, None)
        self.assertTrue(SCANNER, "Run with the pinned GITLEAKS_BIN; do not skip scanner tests")
        self.git("init", "-q")
        self.git("config", "user.name", "Privacy Test")
        self.git("config", "user.email", "test@example.invalid")
        self.git("config", "core.hooksPath", os.devnull)
        (self.repo / ".gitleaksignore").write_text("")
        (self.repo / "README.md").write_text("public source\n")
        self.git("add", ".")
        self.git("commit", "-qm", "initial")
        # Synthetic recognisable token, generated only inside an isolated local fixture.
        self.secret = "gh" + "p_" + "aB3dE6gH9jK2mN5pQ8sT1vW4yZ7cF0iL3oR6uX9z"

    def git(self, *args):
        return subprocess.run(["git", *args], cwd=self.repo, env=self.env,
                              capture_output=True, check=True)

    def run_check(self, history=False, **overrides):
        result = subprocess.run(
            [sys.executable, str(CHECK), "--history" if history else "--staged"],
            cwd=self.repo, env=dict(self.env, **overrides), capture_output=True, text=True,
        )
        self.assertNotIn(self.secret, result.stdout + result.stderr)
        return result.returncode

    def stage(self, name, value):
        path = self.repo / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(value)
        self.git("add", "--", name)

    def test_clean_index_and_history(self):
        self.stage("中文 file with spaces.txt", "legitimate source\n")
        self.assertEqual(self.run_check(), 0)
        self.assertEqual(self.run_check(history=True), 0)

    def test_staged_secret_clean_worktree_is_blocked(self):
        self.stage("settings.txt", self.secret)
        (self.repo / "settings.txt").write_text("clean but not staged")
        self.assertNotEqual(self.run_check(), 0)

    def test_unstaged_secret_does_not_replace_clean_index(self):
        self.stage("settings.txt", "public data")
        (self.repo / "settings.txt").write_text(self.secret)
        self.assertEqual(self.run_check(), 0)

    def test_rename_only_secret_is_blocked(self):
        self.stage("old.txt", self.secret)
        self.git("commit", "-qm", "fixture baseline")
        self.git("mv", "old.txt", "renamed\nfile.txt")
        self.assertNotEqual(self.run_check(), 0)

    def test_removed_secret_still_blocks_history(self):
        self.stage("old.txt", self.secret)
        self.git("commit", "-qm", "fixture baseline")
        self.git("rm", "old.txt")
        self.assertEqual(self.run_check(), 0)
        self.git("commit", "-qm", "remove fixture")
        self.assertNotEqual(self.run_check(history=True), 0)

    def test_symlink_to_regular_file_secret_is_blocked(self):
        path = self.repo / "settings.txt"
        path.symlink_to("README.md")
        self.git("add", "settings.txt")
        self.git("commit", "-qm", "fixture symlink")
        path.unlink()
        self.stage("settings.txt", self.secret)
        self.assertEqual(self.git("diff", "--cached", "--name-status").stdout,
                         b"T\tsettings.txt\n")
        self.assertNotEqual(self.run_check(), 0)

    def test_inline_allow_comment_does_not_bypass(self):
        self.stage("fixture.py", 'token = "' + self.secret + '" # gitleaks:allow\n')
        self.assertNotEqual(self.run_check(), 0)

    def test_alternate_key_format_is_blocked(self):
        other = "AI" + "za" + "SyB8qX3nL0aV7eM2rT9uW4jK6pC1dF5gH8z"
        self.stage("data.json", '{"key":"' + other + '"}')
        self.assertNotEqual(self.run_check(), 0)

    def test_missing_binary_fails_closed(self):
        self.assertNotEqual(self.run_check(GITLEAKS_BIN=str(self.repo / "missing")), 0)

    def test_wrong_version_and_scanner_error_fail_closed(self):
        fake = self.repo / "fake-scanner"
        fake.write_text("#!/bin/sh\necho 0.0.0\n")
        fake.chmod(0o700)
        self.assertNotEqual(self.run_check(GITLEAKS_BIN=str(fake)), 0)
        fake.write_text('#!/bin/sh\nif [ "$1" = version ]; then echo 8.30.1; else exit 7; fi\n')
        self.stage("settings.txt", "public data")
        self.assertNotEqual(self.run_check(GITLEAKS_BIN=str(fake)), 0)

    def test_ambient_config_and_unstaged_policy_cannot_disable_rules(self):
        self.stage("settings.txt", self.secret)
        policy = self.repo / ".gitleaks.toml"
        policy.write_text('[allowlist]\npaths = [".*"]\n')
        self.assertNotEqual(self.run_check(GITLEAKS_CONFIG=str(policy)), 0)

    def test_symlink_is_scanned_as_index_blob_not_followed(self):
        outside = Path(self.temp.name) / "outside"
        outside.write_text(self.secret)
        (self.repo / "linked.txt").symlink_to(outside)
        self.git("add", "linked.txt")
        self.assertEqual(self.run_check(), 0)

    def test_actual_hook_blocks_commit_without_echoing_secret(self):
        target = self.repo / ".github/security/check_secrets.py"
        target.parent.mkdir(parents=True)
        shutil.copyfile(CHECK, target)
        self.git("add", ".github")
        self.git("commit", "-qm", "fixture hook source")
        self.git("config", "core.hooksPath", str(ROOT / ".githooks"))
        self.stage("settings.txt", self.secret)
        result = subprocess.run(["git", "commit", "-qm", "must fail"], cwd=self.repo,
                                env=self.env, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn(self.secret, result.stdout + result.stderr)


@unittest.skipUnless(os.environ.get("ONEHUB_DOCKER_TEST") == "1",
                     "real Docker context verification runs in the CI privacy gate")
class DockerContextTests(unittest.TestCase):
    def test_private_context_is_excluded_and_build_inputs_remain(self):
        required = ["VERSION", "go.mod", "go.sum", "web/package.json", "web/yarn.lock",
                    "web/src/main.jsx", "web/public/favicon.ico", "config.example.yaml",
                    "controller/check_channel/check.png", "common/limit/slidingwindow.lua",
                    "one-api-amd64", "one-api-arm64", "bin/migration_v0.2-v0.3.sql"]
        private = [".git/config", ".env", "web/.env.production.local", "nested/config.yaml",
                   "data/users.db", "nested/users.sqlite3-wal", "logs/debug.log",
                   "backups/dump.sql", "dump.sql", "nested/id_ed25519", "nested/auth.key",
                   "nested/auth.pem", "web/node_modules/private.txt", ".codex/state.json",
                   "web/build/stale.js", "one-api"]
        with tempfile.TemporaryDirectory(prefix="onehub-docker-privacy-") as temp:
            base = Path(temp)
            context = base / "context"
            context.mkdir()
            for name in required + private:
                path = context / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("synthetic Docker context fixture\n")
            for hardened in (False, True):
                if hardened:
                    shutil.copyfile(ROOT / ".dockerignore", context / ".dockerignore")
                dest = base / ("protected" if hardened else "baseline")
                result = subprocess.run(
                    ["docker", "build", "--no-cache", "--file", "-", "--output",
                     "type=local,dest=" + str(dest), str(context)],
                    input=b"FROM scratch\nCOPY . /\n", capture_output=True,
                )
                self.assertEqual(result.returncode, 0, "Docker context export failed")
                for name in required:
                    self.assertTrue((dest / name).is_file(), name)
                for name in private:
                    self.assertEqual((dest / name).exists(), not hardened, name)


if __name__ == "__main__":
    unittest.main()
