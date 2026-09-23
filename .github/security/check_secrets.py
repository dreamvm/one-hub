#!/usr/bin/env python3
"""Fail-closed, fully redacted checks of staged blobs or complete Git history."""
import argparse
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

VERSION = "8.30.1"


def git(repo, *args):
    result = subprocess.run(
        ["git", "-C", str(repo), *args], capture_output=True, check=False
    )
    if result.returncode:
        raise RuntimeError("Git could not read the requested index/history data")
    return result.stdout


def scanner(repo):
    candidate = os.environ.get("GITLEAKS_BIN")
    if not candidate:
        result = subprocess.run(
            ["git", "-C", str(repo), "config", "--path", "--get", "onehub.gitleaksPath"],
            capture_output=True,
        )
        candidate = result.stdout.decode().strip() if result.returncode == 0 else None
    candidate = candidate or shutil.which("gitleaks")
    if not candidate:
        raise RuntimeError("Install Gitleaks 8.30.1; see docs/PRIVACY_GUARDRAILS.md")
    result = subprocess.run([candidate, "version"], capture_output=True)
    if result.returncode or result.stdout.decode().strip() != VERSION:
        raise RuntimeError("Gitleaks must be exactly version 8.30.1")
    return candidate


def check(repo, history=False):
    executable = scanner(repo)
    with tempfile.TemporaryDirectory(prefix="onehub-secret-check-") as temp:
        temp = Path(temp)
        # Do not let ambient GITLEAKS_CONFIG or an unstaged policy change disable rules.
        config = temp / "defaults.toml"
        config.write_text("[extend]\nuseDefault = true\n")
        ignore = temp / "ignore"
        ignore.write_bytes(b"")
        if history:
            if git(repo, "rev-parse", "--is-shallow-repository").strip() != b"false":
                raise RuntimeError("History scan requires a full clone (fetch-depth: 0)")
            # Only committed, immutable upstream fingerprints are accepted in CI.
            ignore.write_bytes(git(repo, "show", "HEAD:.gitleaksignore"))
            target = ["git", str(repo), "--log-opts=--all --full-history"]
        else:
            names = git(
                repo, "diff", "--cached", "--name-only", "--diff-filter=ACMRT", "-z"
            ).split(b"\0")
            names = [os.fsdecode(name) for name in names if name]
            if not names:
                print("Secret check: no added/changed staged files.")
                return 0
            snapshot = temp / "index"
            snapshot.mkdir()
            for name in names:
                path = Path(name)
                if path.is_absolute() or ".." in path.parts:
                    raise RuntimeError("Unsafe index path")
                # Materialize the entire staged blob, not only added diff lines.
                # This catches rename-only files and a cleaned but unstaged worktree.
                # Symlink blobs become regular text; never follow external targets.
                data = git(repo, "cat-file", "blob", ":" + name)
                dest = snapshot / path
                dest.parent.mkdir(parents=True, exist_ok=True)
                dest.write_bytes(data)
            target = ["dir", str(snapshot)]
        result = subprocess.run(
            [executable, *target, "--config", str(config),
             "--gitleaks-ignore-path", str(ignore), "--ignore-gitleaks-allow",
             "--redact=100", "--no-banner", "--no-color", "--exit-code=1",
             "--log-level=error"],
            capture_output=True,
        )
        # Never echo scanner diagnostics: even error paths must not print a secret.
        if result.returncode:
            print("Secret check BLOCKED: sensitive content or a scanner error. "
                  "Review the staged files/history locally; no secret values are printed.",
                  file=sys.stderr)
            return 1
        print("Secret check passed (Gitleaks 8.30.1, redacted).")
        return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--staged", action="store_true")
    mode.add_argument("--history", action="store_true")
    args = parser.parse_args()
    try:
        root = git(Path.cwd(), "rev-parse", "--show-toplevel").decode().strip()
        return check(Path(root), args.history)
    except (OSError, RuntimeError, UnicodeError):
        print("Secret check BLOCKED: setup, version, index or history could not be "
              "verified. See docs/PRIVACY_GUARDRAILS.md.", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
