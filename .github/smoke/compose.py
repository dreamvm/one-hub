#!/usr/bin/env python3
"""Exercise repository Compose templates using only locally loaded synthetic images/data."""

import argparse
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import signal
import subprocess
import tempfile


def run():
    parser = argparse.ArgumentParser()
    parser.add_argument("--compose", required=True)
    parser.add_argument("--image", required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    if not re.fullmatch(r"sha256:[a-f0-9]{64}", args.image):
        raise ValueError("Expected a verified locally loaded candidate image ID")
    root = Path(__file__).resolve().parents[2]
    for mode, files in (
        ("default", ["docker-compose.yml"]),
        ("sqlite", ["docker-compose.sqlite.yml"]),
        ("mysql", ["docker-compose.sqlite.yml", "docker-compose.mysql.yml"]),
        ("sqlite-redis", ["docker-compose.sqlite.yml", "docker-compose.redis.yml"]),
    ):
        with tempfile.TemporaryDirectory(prefix="onehub-compose-") as directory:
            fixture = Path(directory)
            for source in root.glob("docker-compose*.yml"):
                shutil.copyfile(source, fixture / source.name)
            project = "onehub-compose-" + secrets.token_hex(6)
            services = ["one-hub"]
            if mode in ("default", "mysql"):
                services.append("db")
            if mode in ("default", "sqlite-redis"):
                services.append("redis")
            # Keep commands and healthchecks intact. Use disposable volumes, names and network,
            # no host ports, and the locally loaded, unpublished candidate image.
            override = ["services:"]
            for service in services:
                override.extend([f"  {service}:", f"    container_name: {project}-{service}"])
                if service == "one-hub":
                    override.extend([f"    image: {args.image}", "    ports: !reset []",
                                     "    volumes: !override [app-data:/data]",
                                     "    environment:", "      DISABLE_TOKEN_ENCODERS: 'true'",
                                     "      AUTO_PRICE_UPDATES: 'false'"])
                elif service == "db":
                    override.append("    volumes: !override [mysql-data:/var/lib/mysql]")
            override.extend(["volumes:", "  app-data: {}", "  mysql-data: {}",
                             "networks:", "  default:", "    internal: true"])
            (fixture / "fixture.yml").write_text("\n".join(override) + "\n")
            env = os.environ.copy()
            for key in list(env):
                if key.startswith("ONEHUB_") or key.startswith("COMPOSE_"):
                    del env[key]
            env.update({
                "ONEHUB_IMAGE_DIGEST": args.image,
                "ONEHUB_SESSION_SECRET": secrets.token_hex(32),
                "ONEHUB_USER_TOKEN_SECRET": secrets.token_hex(32),
                "ONEHUB_MYSQL_PASSWORD": secrets.token_hex(32),
                "ONEHUB_MYSQL_ROOT_PASSWORD": secrets.token_hex(32),
            })
            command = [args.compose, "--project-name", project, "--env-file", os.devnull]
            for file in [*files, "fixture.yml"]:
                command.extend(["-f", str(fixture / file)])

            def invoke(*options, timeout=180):
                result = subprocess.run([*command, *options], cwd=fixture, env=env,
                                        text=True, capture_output=True, timeout=timeout)
                if result.returncode:
                    # Do not print resolved environment or container logs containing credentials.
                    raise RuntimeError(f"Compose {mode} {options[0]} failed ({result.returncode})")
                return result.stdout

            try:
                invoke("up", "-d", "--wait", "--wait-timeout", "150", "--pull", "never", "--no-build")
                response = json.loads(invoke("exec", "-T", "one-hub", "wget", "-q", "-O", "-",
                                             "http://127.0.0.1:3000/api/status"))
                if response.get("success") is not True or response.get("data", {}).get("version") != args.version:
                    raise AssertionError("Compose candidate status/version mismatch")
                print(f"COMPOSE_OK {mode}: template healthchecks and candidate version", flush=True)
            finally:
                invoke("down", "--volumes", "--remove-orphans")


if __name__ == "__main__":
    def interrupted(signum, _frame):
        raise KeyboardInterrupt(f"interrupted by signal {signum}")

    signal.signal(signal.SIGTERM, interrupted)
    run()
