#!/usr/bin/env python3
"""Isolated image checks. Only synthetic accounts, credentials and upstream data."""

import argparse
import copy
import http.cookies
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import signal
import subprocess
import tempfile
import time
import uuid


NAMES = ["read_doc_metadata", "list_doc_sections"]
SIGNATURES = ["fixture-signature-A", "fixture-signature-B"]


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def command(*args, check=True, input_text=None):
    result = subprocess.run(args, input=input_text, text=True, capture_output=True, timeout=90)
    if check and result.returncode:
        raise RuntimeError(f"{args[0]} {args[1]} failed: {result.stderr[-1500:]}")
    return result


def start_container(name, options, created):
    # Register the container before start: a failed OCI start still leaves it behind.
    command("docker", "create", "--pull=never", "--name", name, *options)
    created.append(("container", name))
    command("docker", "start", name)


class Client:
    def __init__(self, base, container):
        self.base = base
        self.container = container
        self.cookies = http.cookies.SimpleCookie()

    def request(self, path, data=None, token=None, method=None):
        headers = {"Content-Type": "application/json"}
        if token:
            headers["Authorization"] = "Bearer " + token
        if self.cookies:
            headers["Cookie"] = "; ".join(f"{key}={value.value}" for key, value in self.cookies.items())
        result = command("docker", "exec", "-i", self.container, "/fixture/mock", "request", input_text=json.dumps({
            "url": self.base + path, "method": method or ("GET" if data is None else "POST"),
            "headers": headers, "body": "" if data is None else json.dumps(data, ensure_ascii=False),
        }))
        response = json.loads(result.stdout)
        if "error" in response:
            raise OSError(response["error"])
        for value in response["headers"].get("Set-Cookie", []):
            self.cookies.load(value)
        return response["status"], {key: ", ".join(value) for key, value in response["headers"].items()}, response["body"]

    def api(self, path, data=None, method=None):
        status, _, body = self.request(path, data=data, method=method)
        result = json.loads(body)
        require(status == 200 and result.get("success") is True, f"API {path}: {status}: {body[:800]}")
        return result.get("data")


def parse_chat(status, headers, body, stream):
    require(status == 200, f"chat HTTP {status}: {body[:800]}")
    if not stream:
        response = json.loads(body)
        require("error" not in response, f"chat error: {body[:800]}")
        require(response.get("choices"), "missing choices")
        return response["choices"][0]["message"], response.get("usage", {})
    require("text/event-stream" in headers.get("Content-Type", ""), "not SSE")
    chunks = []
    done = False
    for line in body.splitlines():
        if not line.startswith("data: "):
            continue
        data = line[6:]
        if data == "[DONE]":
            done = True
        else:
            require(not done, "data after DONE")
            chunk = json.loads(data)
            require("error" not in chunk, f"SSE error: {data[:800]}")
            chunks.append(chunk)
    require(done and chunks, "truncated or empty SSE")
    message = {"role": "assistant", "content": ""}
    calls, usage, finished = {}, {}, False
    for chunk in chunks:
        if chunk.get("usage"):
            usage = chunk["usage"]
        for choice in chunk.get("choices", []):
            finished |= bool(choice.get("finish_reason"))
            delta = choice.get("delta", {})
            message["content"] += delta.get("content") or ""
            if delta.get("extra_content"):
                message["extra_content"] = delta["extra_content"]
            for part in delta.get("tool_calls", []):
                index = part["index"]
                target = calls.setdefault(index, {"type": "function", "function": {"name": "", "arguments": ""}})
                for key in ("id", "extra_content"):
                    if part.get(key):
                        require(key not in target, f"repeated tool header {index}/{key}")
                        target[key] = part[key]
                for key in ("name", "arguments"):
                    target["function"][key] += part.get("function", {}).get(key) or ""
    require(finished, "missing stream finish reason")
    if calls:
        require(sorted(calls) == list(range(len(calls))), "non-contiguous tool indexes")
        message["tool_calls"] = [calls[i] for i in sorted(calls)]
    return message, usage


def check_tools(message, provider="google"):
    calls = message.get("tool_calls", [])
    require(len(calls) == 2, "expected two tool calls")
    require(len({c.get("id") for c in calls}) == 2 and all(c.get("id") for c in calls), "tool IDs lost or duplicated")
    for i, call in enumerate(calls):
        require(call["function"]["name"] == NAMES[i], "tool name changed")
        require(json.loads(call["function"]["arguments"]) == {"file": "中文测试.docx"}, "tool arguments changed")
        if provider == "google":
            require(call.get("extra_content", {}).get("google", {}).get("thought_signature") == SIGNATURES[i], "signature changed")
    if provider == "anthropic":
        blocks = calls[0].get("extra_content", {}).get("anthropic", {}).get("content", [])
        require(len(blocks) == 4, "Claude signed blocks missing")
        require(blocks[0] == {"type": "thinking", "thinking": "synthetic thought", "signature": "fixture-claude-signature"}, "Claude signature changed")
    return calls


def chat(client, token, model, stream, messages=None, tools=None):
    body = {"model": model, "stream": stream, "max_tokens": 64,
            "messages": messages or [{"role": "user", "content": "中文对话测试"}]}
    if stream:
        body["stream_options"] = {"include_usage": True}
    if tools:
        body["tools"] = tools
    status, headers, raw = client.request("/v1/chat/completions", data=body, token=token)
    message, usage = parse_chat(status, headers, raw, stream)
    require(usage.get("total_tokens", 0) > 0, "missing upstream usage")
    return message


def wait_ready(client):
    deadline = time.monotonic() + 60
    last_error = None
    while time.monotonic() < deadline:
        try:
            return client.api("/api/status")
        except (OSError, ValueError, AssertionError) as exc:
            last_error = exc
            time.sleep(1)
    raise RuntimeError(f"gateway did not start: {last_error}")


def validate_backend(args):
    require(args.backend in ("sqlite", "mysql-redis", "postgres-redis"), "unsupported smoke backend")
    if args.backend != "sqlite":
        database_image = args.mysql_image if args.backend == "mysql-redis" else args.postgres_image
        for image in (database_image, args.redis_image):
            require(isinstance(image, str) and re.fullmatch(r"sha256:[a-f0-9]{64}", image),
                    "Database/Redis require explicit local immutable image IDs; tags and remote pulls are not allowed")


class MySQLRedis:
    """Fresh synthetic services, never a connection to an existing database."""

    def __init__(self, args, prefix, tmp, common, created):
        self.mysql, self.redis = prefix + "-mysql", prefix + "-redis"
        self.database = self.mysql
        self.password, self.redis_password = secrets.token_hex(24), secrets.token_hex(24)
        volume = prefix + "-mysql-data"
        command("docker", "volume", "create", volume)
        created.append(("volume", volume))
        start_container(self.mysql, [*common, "--user", "999:999", "--network-alias", "database",
                        "--memory", "1g", "--cpus", "1", "--mount", f"type=volume,source={volume},target=/var/lib/mysql",
                        "--tmpfs", "/var/run/mysqld:rw,nosuid,size=16m,uid=999,gid=999",
                        "-e", "MYSQL_ROOT_PASSWORD=" + secrets.token_hex(24), "-e", "MYSQL_DATABASE=onehub_smoke",
                        "-e", "MYSQL_USER=smoke", "-e", "MYSQL_PASSWORD=" + self.password,
                        args.mysql_image, "--innodb-buffer-pool-size=128M", "--max-connections=30"], created)
        self.start_redis(args, tmp, common, created)
        self.wait_ready()

    def start_redis(self, args, tmp, common, created):
        config = tmp / "redis.conf"
        config.write_text('bind 0.0.0.0\nprotected-mode yes\nsave ""\nappendonly no\nmaxmemory 64mb\n'
                          'maxmemory-policy noeviction\nrequirepass ' + self.redis_password + '\n', encoding="utf-8")
        config.chmod(0o644)
        start_container(self.redis, [*common, "--user", "999:999", "--network-alias", "cache",
                        "--memory", "128m", "--cpus", "0.25", "--tmpfs", "/data:rw,nosuid,size=16m,uid=999,gid=999",
                        "--mount", f"type=bind,source={config},target=/fixture-redis.conf,readonly",
                        args.redis_image, "redis-server", "/fixture-redis.conf"], created)

    def sql(self, query, check=True):
        return command("docker", "exec", "-i", "-e", "MYSQL_PWD=" + self.password, self.mysql,
                       "mysql", "--protocol=tcp", "--host=127.0.0.1", "--user=smoke", "--database=onehub_smoke",
                       "--batch", "--skip-column-names", check=check, input_text=query)

    def cache(self, *args, check=True):
        return command("docker", "exec", "-e", "REDISCLI_AUTH=" + self.redis_password, self.redis,
                       "redis-cli", "--raw", *args, check=check)

    def wait_ready(self):
        deadline = time.monotonic() + 120
        while time.monotonic() < deadline:
            sql = self.sql("SELECT 1;", check=False)
            cache = self.cache("PING", check=False)
            if sql.returncode == 0 and sql.stdout.strip() == "1" and cache.stdout.strip() == "PONG":
                return
            time.sleep(1)
        raise RuntimeError("isolated database/Redis did not become ready")

    def environment(self):
        return ["-e", f"SQL_DSN=smoke:{self.password}@tcp(database:3306)/onehub_smoke?charset=utf8mb4&parseTime=True&loc=Local",
                "-e", f"REDIS_CONN_STRING=redis://:{self.redis_password}@cache:6379/0",
                "-e", "SYNC_FREQUENCY=600", "-e", "REDIS_DB=0"]

    def assert_used(self, channel_count=3):
        # Counts only: do not print test tokens or session contents.
        require(int(self.sql("SELECT COUNT(*) FROM users;").stdout.strip()) >= 2, "users not persisted in database")
        require(int(self.sql("SELECT COUNT(*) FROM channels;").stdout.strip()) == channel_count, "channels not persisted in database")
        require(int(self.cache("DBSIZE").stdout.strip()) > 0, "Redis cache remained empty")
        stats = dict(line.split(":", 1) for line in self.cache("INFO", "stats").stdout.splitlines() if ":" in line)
        require(int(stats.get("keyspace_hits", "0")) > 0, "Redis was configured but cache hits were not observed")

    def restart(self):
        for name in (self.database, self.redis):
            command("docker", "restart", "--time", "15", name)
        self.wait_ready()


class PostgreSQLRedis(MySQLRedis):
    """Same business checks as MySQL, with an isolated PostgreSQL 18 cluster."""

    def __init__(self, args, prefix, tmp, common, created):
        self.database, self.redis = prefix + "-postgres", prefix + "-redis"
        self.password, self.redis_password = secrets.token_hex(24), secrets.token_hex(24)
        volume = prefix + "-postgres-data"
        command("docker", "volume", "create", volume)
        created.append(("volume", volume))
        start_container(self.database, [*common, "--user", "999:999", "--network-alias", "database",
                        "--memory", "1g", "--cpus", "1", "--mount", f"type=volume,source={volume},target=/var/lib/postgresql",
                        "--tmpfs", "/var/run/postgresql:rw,nosuid,size=16m,uid=999,gid=999",
                        "-e", "POSTGRES_DB=onehub_smoke", "-e", "POSTGRES_USER=smoke",
                        "-e", "POSTGRES_PASSWORD=" + self.password, "-e", "PGDATA=/var/lib/postgresql/18/docker",
                        args.postgres_image, "-c", "shared_buffers=128MB", "-c", "max_connections=30"], created)
        self.start_redis(args, tmp, common, created)
        self.wait_ready()

    def sql(self, query, check=True):
        return command("docker", "exec", "-i", "-e", "PGPASSWORD=" + self.password, self.database,
                       "psql", "--no-psqlrc", "--host=127.0.0.1", "--username=smoke", "--dbname=onehub_smoke",
                       "--tuples-only", "--no-align", "--field-separator=\t", "--set=ON_ERROR_STOP=1",
                       check=check, input_text=query)

    def environment(self):
        return ["-e", f"SQL_DSN=postgres://smoke:{self.password}@database:5432/onehub_smoke?sslmode=disable",
                "-e", f"REDIS_CONN_STRING=redis://:{self.redis_password}@cache:6379/0",
                "-e", "SYNC_FREQUENCY=600", "-e", "REDIS_DB=0"]


def assert_paid_accounting(before, after, unlimited):
    cost = after[1] - before[1]
    require(cost > 0 and before[0] - after[0] == cost, "recovered request quota mismatch")
    require(after[2] == before[2] + 1, "recovered request count mismatch")
    # Unlimited tokens intentionally retain their own quota counters; user accounting still applies.
    token_cost = 0 if unlimited else cost
    require(after[3] - before[3] == token_cost and after[4] == before[4] + 1,
            "recovered request token/ledger mismatch")


def redis_failure_checks(backend, user, token, user_id, mock, passed):
    """Fault only this run's synthetic Redis; verify database and upstream effects."""
    user_id = int(user_id)
    upstream = Client("http://mock-provider:8000", mock)
    mode = backend.sql(f"SELECT CASE WHEN unlimited_quota THEN 1 ELSE 0 END FROM tokens "
                       f"WHERE user_id={user_id} AND name='sys_playground';").stdout.strip()
    require(mode in ("0", "1"), "expected exactly one playground token")
    unlimited = mode == "1"

    def snapshot():
        query = (f"SELECT quota,used_quota,request_count,"
                 f"(SELECT COALESCE(SUM(used_quota),0) FROM tokens WHERE user_id={user_id}),"
                 f"(SELECT COUNT(*) FROM quota_reservations WHERE user_id={user_id}) "
                 f"FROM users WHERE id={user_id};")
        return tuple(int(value) for value in backend.sql(query).stdout.strip().split("\t"))

    def counter():
        code, _, body = upstream.request("/stats")
        require(code == 200, "mock stats unavailable")
        return json.loads(body).get("openai-smoke_false", 0)

    def refused():
        before, calls = snapshot(), counter()
        code, _, _ = user.request("/v1/chat/completions", token=token,
                                 data={"model": "openai-smoke", "messages": [{"role": "user", "content": "fault fixture"}]})
        require(400 <= code < 600, "fault request unexpectedly succeeded")
        require(counter() == calls, "refused request reached upstream")
        require(snapshot() == before, "refused request changed persistent accounting")

    def paid_control():
        before = snapshot()
        require(chat(user, token, "openai-smoke", False)["content"] == "中文对话成功", "recovered chat failed")
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            after = snapshot()
            if after[2] == before[2] + 1:
                assert_paid_accounting(before, after, unlimited)
                return
            time.sleep(0.1)
        raise AssertionError("recovered request did not settle exactly once")

    # Establish a completed paid control before taking failure snapshots.
    paid_control()
    command("docker", "stop", "--time", "1", backend.redis)
    try:
        refused()
    finally:
        command("docker", "start", backend.redis)
        backend.wait_ready()
    paid_control()
    passed("Redis stopped: rejected before upstream/accounting; empty-cache restart recovers paid chat")

    group_key = f"user_group:{user_id}"
    backend.cache("DEL", group_key)
    backend.cache("LPUSH", group_key, "fixture-wrong-type")
    try:
        refused()
    finally:
        backend.cache("DEL", group_key)
    paid_control()
    passed("Redis wrong-type group cache: rejected without side effects; cache miss refills")

    quota_key = f"user_quota:{user_id}"
    backend.cache("SET", quota_key, "0")
    paid_control()
    passed("stale zero Redis quota does not override positive database balance")

    balance = snapshot()[0]
    backend.sql(f"UPDATE users SET quota=0 WHERE id={user_id};")
    backend.cache("SET", quota_key, "999999999")
    try:
        refused()
    finally:
        backend.sql(f"UPDATE users SET quota={balance} WHERE id={user_id};")
        backend.cache("DEL", quota_key)
    paid_control()
    passed("stale high Redis quota cannot authorize zero database balance; restored balance recovers")


def run(args):
    require(re.fullmatch(r"onehub-isolated-smoke:[a-f0-9]{40}", args.image), "only the local smoke image is allowed")
    validate_backend(args)
    prefix = "onehub-smoke-" + uuid.uuid4().hex[:12]
    gateway, mock = prefix + "-gateway", prefix + "-mock"
    network, volume = prefix + "-net", prefix + "-data"
    created, checks = [], []
    backend = None
    print(f"FIXTURE: {prefix}; backend={args.backend}", flush=True)

    def passed(label):
        checks.append(label)
        print("PASS:", label, flush=True)

    with tempfile.TemporaryDirectory(prefix="onehub-smoke-") as tmp:
        tmp = Path(tmp)
        shutil.copy2(args.mock_binary, tmp / "mock")
        command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
                "-subj", "/CN=mock-provider", "-addext", "subjectAltName=DNS:mock-provider",
                "-keyout", str(tmp / "server.key"), "-out", str(tmp / "server.crt"))
        (tmp / "server.crt").chmod(0o644)
        try:
            command("docker", "network", "create", "--internal", network)
            created.append(("network", network))
            require(json.loads(command("docker", "network", "inspect", network).stdout)[0]["Internal"], "network is not internal")
            command("docker", "volume", "create", volume)
            created.append(("volume", volume))
            common = ["--network", network, "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--pids-limit", "256", "--read-only", "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m"]
            if args.backend != "sqlite":
                backend_type = MySQLRedis if args.backend == "mysql-redis" else PostgreSQLRedis
                backend = backend_type(args, prefix, tmp, common, created)
                passed(f"isolated authenticated {args.backend} startup, fresh data volume and resource limits")
            # Both containers use the actual built image. Only the mock entrypoint differs.
            start_container(mock, [*common, "--user", f"{os.getuid()}:{os.getgid()}",
                    "--network-alias", "mock-provider", "--memory", "128m", "--cpus", "0.25",
                    "--mount", f"type=bind,source={tmp},target=/fixture,readonly",
                    "--entrypoint", "/fixture/mock", args.image], created)
            start_container(gateway, [*common,
                    "--network-alias", "gateway",
                    "--memory", "512m", "--cpus", "1", "--mount", f"type=volume,source={volume},target=/data",
                    "--mount", f"type=bind,source={tmp / 'server.crt'},target=/fixture-ca.crt,readonly",
                    "-e", "SSL_CERT_FILE=/fixture-ca.crt", "-e", "DISABLE_TOKEN_ENCODERS=true",
                    "-e", "AUTO_PRICE_UPDATES=false", "-e", "RELAY_TIMEOUT=15", "-e", "CONNECT_TIMEOUT=3",
                    "-e", "SESSION_SECRET=" + secrets.token_hex(32), "-e", "USER_TOKEN_SECRET=" + secrets.token_hex(32),
                    *(backend.environment() if backend else []),
                    args.image], created)
            for kind, name in created:
                if kind != "container":
                    continue
                config = json.loads(command("docker", "inspect", name).stdout)[0]
                require(not config["HostConfig"]["PortBindings"], "unexpected published port")
                require(set(config["NetworkSettings"]["Networks"]) == {network}, "container joined another network")
                require(config["HostConfig"]["Memory"] > 0 and config["HostConfig"]["NanoCpus"] > 0, "resource limits missing")
            client = Client("http://gateway:3000", mock)
            require(wait_ready(client)["version"] == args.version, "wrong binary version")
            passed(f"startup, fresh {args.backend} migration and binary version")
            command("docker", "exec", gateway, "test", "-s", "/etc/ssl/certs/ca-certificates.crt")
            if backend:
                command("docker", "exec", gateway, "test", "!", "-e", "/data/one-api.db")
            else:
                command("docker", "exec", gateway, "test", "-s", "/data/one-api.db")
            status, _, body = client.request("/")
            require(status == 200 and '<div id="root"' in body, "frontend index missing")
            scripts = re.findall(r'<script[^>]+src="([^\"]+)"', body)
            require(scripts, "no bundled script")
            for script in scripts:
                require(script.startswith("/assets/"), "unexpected script path")
                code, headers, asset = client.request(script)
                require(code == 200 and len(asset) > 100 and "javascript" in headers.get("Content-Type", ""), "asset unavailable")
            passed("embedded frontend index/assets and CA bundle")
            status, _, _ = client.request("/v1/models")
            require(status == 401, "unauthenticated model API accepted")
            client.api("/api/user/login", {"username": "root", "password": "123456"})
            password = secrets.token_hex(8)
            client.api("/api/user/self", {"username": "root", "password": password, "display_name": "Smoke Root"}, method="PUT")
            for name, kind, model, base, key in [
                ("Mock OpenAI", 1, "openai-smoke", "http://mock-provider:8000", "fixture-openai-key"),
                ("Mock TLS", 1, "openai-https-smoke", "https://mock-provider:8443", "fixture-openai-key"),
                ("Mock Gemini", 25, "gemini-smoke", "http://mock-provider:8000", "fixture-gemini-key"),
                ("Mock Claude", 14, "claude-smoke", "http://mock-provider:8000", "fixture-claude-key"),
            ]:
                client.api("/api/channel/", {"name": name, "type": kind, "models": model, "key": key, "base_url": base, "group": "default", "status": 1})
            token = client.api("/api/token/playground")
            models_status, _, models_raw = client.request("/v1/models", token=token)
            require(models_status == 200, "model list failed")
            require({m["id"] for m in json.loads(models_raw)["data"]} == {"openai-smoke", "openai-https-smoke", "gemini-smoke", "claude-smoke"}, "unexpected model list")
            for model in ("openai-smoke", "openai-https-smoke"):
                for stream in (False, True):
                    require(chat(client, token, model, stream)["content"] == "中文对话成功", "chat content changed")
            passed("DNS by upstream hostname; HTTP/verified HTTPS; JSON/SSE Chinese chat and usage")
            tools = [{"type": "function", "function": {"name": name, "description": "Synthetic read-only fixture; no files are accessed.", "parameters": {"type": "object", "properties": {"file": {"type": "string"}}, "required": ["file"]}}} for name in NAMES]
            initial = [{"role": "user", "content": "请调用两个只读工具检查中文测试.docx"}]
            for stream in (False, True):
                assistant = chat(client, token, "gemini-smoke", stream, initial, tools)
                calls = check_tools(assistant)
                followup = initial + [assistant] + [{"role": "tool", "tool_call_id": call["id"], "content": '{"ok":true}'} for call in calls]
                require(chat(client, token, "gemini-smoke", stream, followup, tools)["content"] == "工具签名往返成功", "tool follow-up failed")
            passed("Gemini two-tool IDs/arguments/signatures and tool-result follow-up, JSON and SSE")
            # Negative control: the mock must reject a corrupted signature through the real relay.
            broken = copy.deepcopy(followup)
            broken[1]["tool_calls"][0]["extra_content"]["google"]["thought_signature"] = "corrupted-fixture"
            code, _, raw = client.request("/v1/chat/completions", token=token, data={"model": "gemini-smoke", "messages": broken, "tools": tools})
            require(code >= 400 and "tool signature" in raw, "corrupt signature was accepted")
            passed("negative control: corrupted signature rejected by upstream")
            def claude_roundtrip(actor, actor_token, stream):
                assistant = chat(actor, actor_token, "claude-smoke", stream, initial, tools)
                calls = check_tools(assistant, "anthropic")
                messages = initial + [assistant] + [{"role": "tool", "tool_call_id": call["id"], "content": '{"ok":true}'} for call in calls]
                require(chat(actor, actor_token, "claude-smoke", stream, messages, tools)["content"] == "Claude工具往返成功", "Claude follow-up failed")
            for stream in (False, True):
                claude_roundtrip(client, token, stream)
            passed("Claude signed thinking/text/two-tool round-trip with fragmented JSON arguments, JSON and SSE")
            user_password = secrets.token_hex(8)
            client.api("/api/user/", {"username": "smokeuser", "password": user_password})
            user = Client(client.base, mock)
            user_info = user.api("/api/user/login", {"username": "smokeuser", "password": user_password})
            require(user_info["role"] == 1, "not a normal user")
            client.api(f"/api/user/quota/{user_info['id']}", {"quota": 1000000, "remark": "synthetic smoke quota"})
            user_token = user.api("/api/token/playground")
            require(chat(user, user_token, "openai-smoke", False)["content"] == "中文对话成功", "ordinary user chat failed")
            check_tools(chat(user, user_token, "gemini-smoke", True, initial, tools))
            claude_roundtrip(user, user_token, True)
            _, _, denied = user.request("/api/channel/")
            require(json.loads(denied).get("success") is False, "ordinary user received admin access")
            passed("ordinary-user chat/tools with admin access denied")
            if backend:
                backend.assert_used(channel_count=4)
                passed(f"{args.backend} user/channel persistence and verified Redis cache hits")
                redis_failure_checks(backend, user, user_token, user_info["id"], mock, passed)
                command("docker", "stop", "--time", "10", gateway)
                backend.restart()
                command("docker", "start", gateway)
            else:
                command("docker", "restart", "--time", "10", gateway)
            require(wait_ready(client)["version"] == args.version, "restart version changed")
            login = Client(client.base, mock)
            login.api("/api/user/login", {"username": "root", "password": password})
            require(chat(user, user_token, "openai-smoke", False)["content"] == "中文对话成功", "persisted user/token/channel failed after restart")
            if backend:
                # Redis has persistence disabled. A restarted empty cache must refill from the database.
                check_tools(chat(user, user_token, "gemini-smoke", True, initial, tools))
                backend.assert_used(channel_count=4)
            passed(f"restart persistence: {args.backend}, changed password, users, tokens and channels")
            if backend:
                token_id = int(backend.sql(f"SELECT id FROM tokens WHERE user_id={int(user_info['id'])} AND name='sys_playground';").stdout.strip())
                user.api(f"/api/token/{token_id}", method="DELETE")
                code, _, _ = user.request("/v1/chat/completions", token=user_token,
                                           data={"model": "openai-smoke", "messages": [{"role": "user", "content": "revoked token check"}]})
                require(code in (401, 403), "deleted token was still authorized after Redis cache warmup")
                passed("revoked ordinary-user token rejected after Redis cache warmup")
            _, _, stats_raw = Client("http://mock-provider:8000", mock).request("/stats")
            stats = json.loads(stats_raw)
            for key in ("openai-smoke_false", "openai-smoke_true", "openai-https-smoke_false", "openai-https-smoke_true", "gemini_tools_false", "gemini_tools_true", "gemini_followup_false", "gemini_followup_true", "claude_tools_false", "claude_tools_true", "claude_followup_false", "claude_followup_true", "rejected"):
                require(stats.get(key, 0) > 0, f"upstream branch not exercised: {key}")
            print("Synthetic upstream counters:", json.dumps(stats, sort_keys=True), flush=True)
        except BaseException:
            for kind, name in created:
                if kind == "container":
                    diagnostic = command('docker', 'logs', '--tail', '60', name, check=False)
                    print(f"Diagnostics for {name}:\n{diagnostic.stdout}{diagnostic.stderr}", flush=True)
            raise
        finally:
            failures = []
            for kind, name in reversed(created):
                args_rm = ["docker", "rm", "-f", name] if kind == "container" else ["docker", kind, "rm", name]
                if command(*args_rm, check=False).returncode:
                    failures.append(name)
            require(not failures, "cleanup failed: " + ", ".join(failures))
            print("CLEANUP: removed this run's containers, temporary data volumes and internal network", flush=True)
    passed("temporary resources cleaned up")
    summary = "## Isolated image smoke passed\n\n" + "\n".join("- " + item for item in checks)
    summary += f"\n\nSynthetic upstream only; backend={args.backend}; token encoders disabled for offline startup. No real models, Office generation, production data, registry push or deployment.\n"
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as report:
            report.write(summary)


if __name__ == "__main__":
    def interrupted(signum, _frame):
        raise KeyboardInterrupt(f"interrupted by signal {signum}")

    signal.signal(signal.SIGTERM, interrupted)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", required=True)
    parser.add_argument("--mock-binary", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--backend", choices=("sqlite", "mysql-redis", "postgres-redis"), default="sqlite")
    parser.add_argument("--mysql-image", help="Preloaded immutable MySQL image ID; mysql user must have UID 999")
    parser.add_argument("--postgres-image", help="Preloaded immutable PostgreSQL 18 image ID; postgres user must have UID 999")
    parser.add_argument("--redis-image", help="Preloaded immutable Redis image ID; redis user must have UID 999")
    run(parser.parse_args())
