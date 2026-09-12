#!/usr/bin/env python3
"""Synthetic MySQL/Redis upgrade and rollback rehearsal; no production connections."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import signal
import tempfile
import uuid

from run import Client, MySQLRedis, NAMES, chat, check_tools, command, require, start_container, wait_ready


def fixture_password():
    # Both v0.14.27 and the maintained version validate passwords at 8..20 characters.
    return secrets.token_hex(8)


def validate_images(args):
    for image in (args.old_image, args.candidate_image, args.mysql_image, args.redis_image):
        require(re.fullmatch(r"sha256:[a-f0-9]{64}", image or ""), "only preloaded immutable image IDs are allowed")
    require(args.old_image != args.candidate_image, "upgrade images must differ")


def fixture_target(backend, created):
    require(re.fullmatch(r"onehub-upgrade-[a-f0-9]{12}-mysql", backend.mysql), "not an isolated upgrade database")
    require(("container", backend.mysql) in created, "database was not created by this run")


def database_dump(backend, created):
    fixture_target(backend, created)
    result = command("docker", "exec", "-e", "MYSQL_PWD=" + backend.password, backend.mysql,
                     "mysqldump", "--protocol=tcp", "--host=127.0.0.1", "--user=smoke", "--single-transaction",
                     "--no-tablespaces", "--set-gtid-purged=OFF", "--skip-comments", "--skip-lock-tables",
                     "--order-by-primary", "--add-drop-database", "--databases", "onehub_smoke")
    require("CREATE DATABASE" in result.stdout and "CREATE TABLE" in result.stdout, "incomplete synthetic backup")
    return result.stdout


def restore_database(backend, created, backup, expected_digest):
    fixture_target(backend, created)
    data = backup.read_text(encoding="utf-8")
    require(hashlib.sha256(data.encode()).hexdigest() == expected_digest, "backup checksum mismatch")
    backend.sql(data)
    require(database_dump(backend, created) == data, "restored schema/data differ from the pre-upgrade dump")


def critical_records(backend):
    # Hold values in memory for comparison; never print passwords, test keys or records.
    queries = {
        "users": "SELECT id,username,password,role,status,display_name,quota,used_quota,`group` FROM users ORDER BY id;",
        "tokens": "SELECT id,user_id,`key`,status,name,expired_time,remain_quota,unlimited_quota,used_quota,`group`,backup_group,setting FROM tokens ORDER BY id;",
        "channels": "SELECT id,type,`key`,status,name,base_url,models,`group`,model_mapping,priority,used_quota FROM channels ORDER BY id;",
    }
    return {name: backend.sql(query).stdout for name, query in queries.items()}


def column_names(backend):
    rows = backend.sql("SELECT CONCAT(TABLE_NAME,'.',COLUMN_NAME) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='onehub_smoke' ORDER BY TABLE_NAME,ORDINAL_POSITION;")
    return set(rows.stdout.splitlines())


def basic_checks(mock, password, user_password, token, disabled_token, phase):
    root = Client("http://gateway:3000", mock)
    root.api("/api/user/login", {"username": "root", "password": password})
    user = Client(root.base, mock)
    profile = user.api("/api/user/login", {"username": "upgradeuser", "password": user_password})
    require(profile["role"] == 1, f"{phase}: ordinary user role changed")
    for model in ("openai-smoke", "openai-https-smoke"):
        for stream in (False, True):
            require(chat(user, token, model, stream)["content"] == "中文对话成功", f"{phase}: existing token/channel failed")
    _, _, denied = user.request("/api/channel/")
    require(json.loads(denied).get("success") is False, f"{phase}: admin permission leaked")
    code, _, _ = user.request("/v1/chat/completions", token=disabled_token,
                              data={"model": "openai-smoke", "messages": [{"role": "user", "content": "disabled check"}]})
    require(code in (401, 403), f"{phase}: disabled token became usable")
    return root, user


def run_upgrade(args):
    validate_images(args)
    prefix = "onehub-upgrade-" + uuid.uuid4().hex[:12]
    gateway, mock, network, volume = (prefix + suffix for suffix in ("-gateway", "-mock", "-net", "-data"))
    created, results = [], []
    print("FIXTURE:", prefix, flush=True)

    def passed(label):
        results.append(label)
        print("PASS:", label, flush=True)

    with tempfile.TemporaryDirectory(prefix=prefix + "-") as temporary:
        tmp = Path(temporary)
        shutil.copy2(args.mock_binary, tmp / "mock")
        command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1", "-subj", "/CN=mock-provider",
                "-addext", "subjectAltName=DNS:mock-provider", "-keyout", str(tmp / "server.key"), "-out", str(tmp / "server.crt"))
        (tmp / "server.crt").chmod(0o644)
        try:
            command("docker", "network", "create", "--internal", network)
            created.append(("network", network))
            require(json.loads(command("docker", "network", "inspect", network).stdout)[0]["Internal"], "network not internal")
            command("docker", "volume", "create", volume)
            created.append(("volume", volume))
            common = ["--network", network, "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
                      "--pids-limit", "256", "--read-only", "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m"]
            backend = MySQLRedis(args, prefix, tmp, common, created)
            start_container(mock, [*common, "--user", f"{os.getuid()}:{os.getgid()}", "--network-alias", "mock-provider",
                            "--memory", "128m", "--cpus", "0.25", "--mount", f"type=bind,source={tmp},target=/fixture,readonly",
                            "--entrypoint", "/fixture/mock", args.candidate_image], created)
            stable_environment = [*backend.environment(), "-e", "SSL_CERT_FILE=/fixture-ca.crt", "-e", "DISABLE_TOKEN_ENCODERS=true",
                                  "-e", "AUTO_PRICE_UPDATES=false", "-e", "BATCH_UPDATE_ENABLED=false", "-e", "RELAY_TIMEOUT=15",
                                  "-e", "CONNECT_TIMEOUT=3", "-e", "SESSION_SECRET=" + secrets.token_hex(32),
                                  "-e", "USER_TOKEN_SECRET=" + secrets.token_hex(32)]

            def stop_gateway():
                if ("container", gateway) in created:
                    command("docker", "stop", "--time", "10", gateway)
                    command("docker", "rm", gateway)
                    created.remove(("container", gateway))

            def start_gateway(image, version):
                stop_gateway()
                # This cache belongs only to the synthetic fixture; no live Redis target is accepted.
                require(backend.cache("FLUSHDB").stdout.strip() == "OK", "cannot reset fixture cache")
                start_container(gateway, [*common, "--network-alias", "gateway", "--memory", "512m", "--cpus", "1",
                                "--mount", f"type=volume,source={volume},target=/data",
                                "--mount", f"type=bind,source={tmp / 'server.crt'},target=/fixture-ca.crt,readonly",
                                *stable_environment, image], created)
                for kind, name in created:
                    if kind != "container":
                        continue
                    item = json.loads(command("docker", "inspect", name).stdout)[0]
                    require(not item["HostConfig"]["PortBindings"], "unexpected published port")
                    require(set(item["NetworkSettings"]["Networks"]) == {network}, "unexpected shared network")
                    require(item["HostConfig"]["Memory"] > 0 and item["HostConfig"]["NanoCpus"] > 0, "resource limits missing")
                client = Client("http://gateway:3000", mock)
                require(wait_ready(client)["version"] == version, "wrong running binary version")
                command("docker", "exec", gateway, "test", "!", "-e", "/data/one-api.db")
                return client

            root = start_gateway(args.old_image, args.old_version)
            root.api("/api/user/login", {"username": "root", "password": "123456"})
            password, user_password = fixture_password(), fixture_password()
            root.api("/api/user/self", {"username": "root", "password": password, "display_name": "模拟升级管理员"}, method="PUT")
            for name, kind, model, base, key in [
                ("Mock OpenAI", 1, "openai-smoke", "http://mock-provider:8000", "fixture-openai-key"),
                ("Mock TLS", 1, "openai-https-smoke", "https://mock-provider:8443", "fixture-openai-key"),
                ("Mock Gemini", 25, "gemini-smoke", "http://mock-provider:8000", "fixture-gemini-key"),
            ]:
                root.api("/api/channel/", {"name": name, "type": kind, "models": model, "key": key, "base_url": base, "group": "default", "status": 1})
            root.api("/api/user/", {"username": "upgradeuser", "password": user_password})
            user = Client(root.base, mock)
            profile = user.api("/api/user/login", {"username": "upgradeuser", "password": user_password})
            root.api(f"/api/user/quota/{profile['id']}", {"quota": 1000000, "remark": "synthetic upgrade quota"})
            token = user.api("/api/token/playground")
            user.api("/api/token/", {"name": "disabled-fixture", "remain_quota": 12345, "unlimited_quota": False, "expired_time": -1})
            row = backend.sql("SELECT id,`key` FROM tokens WHERE name='disabled-fixture';").stdout.strip().split("\t")
            disabled_token = row[1]
            user.api("/api/token/?status_only=true", {"id": int(row[0]), "status": 2}, method="PUT")
            basic_checks(mock, password, user_password, token, disabled_token, "old baseline")
            backend.assert_used()
            passed("old image: login, active/disabled tokens, Chinese JSON/SSE, HTTP/TLS, ordinary-user permissions")

            stop_gateway()
            baseline_records, baseline_columns = critical_records(backend), column_names(backend)
            dump = database_dump(backend, created)
            backup = tmp / "pre-upgrade.sql"
            backup.write_text(dump, encoding="utf-8")
            backup.chmod(0o600)
            digest = hashlib.sha256(dump.encode()).hexdigest()
            passed("stopped-writer synthetic database backup and SHA-256 checksum")

            start_gateway(args.candidate_image, args.candidate_version)
            require(critical_records(backend) == baseline_records, "upgrade changed pre-existing identity, quota, token or channel records")
            added_columns = sorted(column_names(backend) - baseline_columns)
            removed_columns = sorted(baseline_columns - column_names(backend))
            require(not removed_columns, "upgrade removed existing columns")
            print("SCHEMA:", json.dumps({"added_columns": added_columns, "removed_columns": removed_columns}), flush=True)
            root, user = basic_checks(mock, password, user_password, token, disabled_token, "upgraded")
            tools = [{"type": "function", "function": {"name": name, "parameters": {"type": "object", "properties": {"file": {"type": "string"}}, "required": ["file"]}}} for name in NAMES]
            initial = [{"role": "user", "content": "检查中文测试.docx"}]
            for stream in (False, True):
                assistant = chat(user, token, "gemini-smoke", stream, initial, tools)
                calls = check_tools(assistant)
                followup = initial + [assistant] + [{"role": "tool", "tool_call_id": call["id"], "content": '{"ok":true}'} for call in calls]
                require(chat(user, token, "gemini-smoke", stream, followup, tools)["content"] == "工具签名往返成功", "upgraded tool round-trip failed")
            root.api("/api/user/", {"username": "postupgrade", "password": fixture_password()})
            passed("upgrade: original records preserved, old credentials/permissions usable, tool signatures round-trip, new writes accepted")

            # Image-only downgrade is evidence for these fixtures, not a universal rollback promise.
            start_gateway(args.old_image, args.old_version)
            basic_checks(mock, password, user_password, token, disabled_token, "image-only downgrade")
            require(backend.sql("SELECT COUNT(*) FROM users WHERE username='postupgrade';").stdout.strip() == "1", "image-only downgrade lost a new row")
            passed("image-only downgrade: old login/tokens/permissions work with upgraded database and post-upgrade row")

            stop_gateway()
            restore_database(backend, created, backup, digest)
            require(critical_records(backend) == baseline_records, "backup restore changed critical records")
            require(column_names(backend) == baseline_columns, "backup restore did not restore the old schema")
            require(backend.sql("SELECT COUNT(*) FROM users WHERE username='postupgrade';").stdout.strip() == "0", "restore did not return to pre-upgrade data")
            passed("backup rollback: entire schema/data dump matches pre-upgrade snapshot; post-upgrade writes removed as expected")
            start_gateway(args.old_image, args.old_version)
            basic_checks(mock, password, user_password, token, disabled_token, "restored old image")
            backend.assert_used()
            passed("restored old image: passwords/tokens/channels/permissions and Redis cache recover")
        except BaseException:
            # Do not dump container logs to the terminal; emit fixture names for correlation.
            print("FAILED: isolated resources scheduled for cleanup:", [name for kind, name in created if kind == "container"], flush=True)
            raise
        finally:
            failures = []
            for kind, name in reversed(created):
                removal = ["docker", "rm", "-f", name] if kind == "container" else ["docker", kind, "rm", name]
                if command(*removal, check=False).returncode:
                    failures.append(name)
            require(not failures, "cleanup failed: " + ", ".join(failures))
            print("CLEANUP: removed only this rehearsal's containers, data volumes and internal network", flush=True)
    passed("synthetic backup and temporary resources cleaned")
    print("RESULT:", json.dumps({"checks_passed": len(results), "real_model_calls": 0, "production_data_copied": False}), flush=True)
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        summary = "## Isolated upgrade and rollback passed\n\n" + "\n".join("- " + item for item in results)
        summary += "\n\nSynthetic data only. No production connections, real model calls, image publication or deployment. "
        summary += "Image-only downgrade is validated only for these fixtures; backup restore discards later writes.\n"
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as report:
            report.write(summary)


if __name__ == "__main__":
    def interrupted(signum, _frame):
        raise KeyboardInterrupt(f"interrupted by signal {signum}")

    signal.signal(signal.SIGTERM, interrupted)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--old-image", required=True)
    parser.add_argument("--old-version", required=True)
    parser.add_argument("--candidate-image", required=True)
    parser.add_argument("--candidate-version", required=True)
    parser.add_argument("--mysql-image", required=True)
    parser.add_argument("--redis-image", required=True)
    parser.add_argument("--mock-binary", type=Path, required=True)
    run_upgrade(parser.parse_args())
