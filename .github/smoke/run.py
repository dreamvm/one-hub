#!/usr/bin/env python3
"""Isolated image checks. Only synthetic accounts, credentials and upstream data."""

import argparse
import copy
import http.cookiejar
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
import urllib.error
import urllib.request
import uuid


NAMES = ["read_doc_metadata", "list_doc_sections"]
SIGNATURES = ["fixture-signature-A", "fixture-signature-B"]


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def command(*args, check=True):
    result = subprocess.run(args, text=True, capture_output=True, timeout=90)
    if check and result.returncode:
        raise RuntimeError(f"{args[0]} {args[1]} failed: {result.stderr[-1500:]}")
    return result


def start_container(name, options, created):
    # Register the container before start: a failed OCI start still leaves it behind.
    command("docker", "create", "--name", name, *options)
    created.append(("container", name))
    command("docker", "start", name)


class Client:
    def __init__(self, base):
        self.base = base
        self.opener = urllib.request.build_opener(
            urllib.request.ProxyHandler({}),
            urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()),
        )

    def request(self, path, data=None, token=None, method=None):
        headers = {"Content-Type": "application/json"}
        if token:
            headers["Authorization"] = "Bearer " + token
        body = None if data is None else json.dumps(data, ensure_ascii=False).encode()
        request = urllib.request.Request(self.base + path, data=body, headers=headers, method=method)
        try:
            with self.opener.open(request, timeout=20) as response:
                return response.status, response.headers, response.read().decode()
        except urllib.error.HTTPError as exc:
            return exc.code, exc.headers, exc.read().decode()

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


def check_tools(message):
    calls = message.get("tool_calls", [])
    require(len(calls) == 2, "expected two tool calls")
    require(len({c.get("id") for c in calls}) == 2 and all(c.get("id") for c in calls), "tool IDs lost or duplicated")
    for i, call in enumerate(calls):
        require(call["function"]["name"] == NAMES[i], "tool name changed")
        require(json.loads(call["function"]["arguments"]) == {"file": "中文测试.docx"}, "tool arguments changed")
        require(call.get("extra_content", {}).get("google", {}).get("thought_signature") == SIGNATURES[i], "signature changed")
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


def run(args):
    require(re.fullmatch(r"onehub-isolated-smoke:[a-f0-9]{40}", args.image), "only the local smoke image is allowed")
    prefix = "onehub-smoke-" + uuid.uuid4().hex[:12]
    gateway, mock = prefix + "-gateway", prefix + "-mock"
    network, volume = prefix + "-net", prefix + "-data"
    created, checks = [], []

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
            # Both containers use the actual built image. Only the mock entrypoint differs.
            start_container(mock, [*common, "--user", f"{os.getuid()}:{os.getgid()}",
                    "--network-alias", "mock-provider", "--memory", "128m", "--cpus", "1",
                    "--mount", f"type=bind,source={tmp},target=/fixture,readonly",
                    "-p", "127.0.0.1::8000", "--entrypoint", "/fixture/mock", args.image], created)
            start_container(gateway, [*common,
                    "--memory", "1g", "--cpus", "2", "--mount", f"type=volume,source={volume},target=/data",
                    "--mount", f"type=bind,source={tmp / 'server.crt'},target=/fixture-ca.crt,readonly",
                    "-e", "SSL_CERT_FILE=/fixture-ca.crt", "-e", "DISABLE_TOKEN_ENCODERS=true",
                    "-e", "AUTO_PRICE_UPDATES=false", "-e", "RELAY_TIMEOUT=15", "-e", "CONNECT_TIMEOUT=3",
                    "-e", "SESSION_SECRET=" + secrets.token_hex(32), "-e", "USER_TOKEN_SECRET=" + secrets.token_hex(32),
                    "-p", "127.0.0.1::3000", args.image], created)
            address = command("docker", "port", gateway, "3000/tcp").stdout.strip()
            mock_address = command("docker", "port", mock, "8000/tcp").stdout.strip()
            require(address.startswith("127.0.0.1:") and mock_address.startswith("127.0.0.1:"), "port not loopback-only")
            client = Client("http://" + address)
            require(wait_ready(client)["version"] == args.version, "wrong binary version")
            passed("startup, fresh SQLite migration and binary version")
            command("docker", "exec", gateway, "test", "-s", "/etc/ssl/certs/ca-certificates.crt")
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
            ]:
                client.api("/api/channel/", {"name": name, "type": kind, "models": model, "key": key, "base_url": base, "group": "default", "status": 1})
            token = client.api("/api/token/playground")
            models_status, _, models_raw = client.request("/v1/models", token=token)
            require(models_status == 200, "model list failed")
            require({m["id"] for m in json.loads(models_raw)["data"]} == {"openai-smoke", "openai-https-smoke", "gemini-smoke"}, "unexpected model list")
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
            user_password = secrets.token_hex(8)
            client.api("/api/user/", {"username": "smokeuser", "password": user_password})
            user = Client(client.base)
            user_info = user.api("/api/user/login", {"username": "smokeuser", "password": user_password})
            require(user_info["role"] == 1, "not a normal user")
            client.api(f"/api/user/quota/{user_info['id']}", {"quota": 1000000, "remark": "synthetic smoke quota"})
            user_token = user.api("/api/token/playground")
            require(chat(user, user_token, "openai-smoke", False)["content"] == "中文对话成功", "ordinary user chat failed")
            check_tools(chat(user, user_token, "gemini-smoke", True, initial, tools))
            _, _, denied = user.request("/api/channel/")
            require(json.loads(denied).get("success") is False, "ordinary user received admin access")
            passed("ordinary-user chat/tools with admin access denied")
            command("docker", "restart", "--time", "10", gateway)
            require(wait_ready(client)["version"] == args.version, "restart version changed")
            login = Client(client.base)
            login.api("/api/user/login", {"username": "root", "password": password})
            require(chat(user, user_token, "openai-smoke", False)["content"] == "中文对话成功", "persisted user/token/channel failed after restart")
            passed("restart persistence: SQLite, changed password, users, tokens and channels")
            _, _, stats_raw = Client("http://" + mock_address).request("/stats")
            stats = json.loads(stats_raw)
            for key in ("openai-smoke_false", "openai-smoke_true", "openai-https-smoke_false", "openai-https-smoke_true", "gemini_tools_false", "gemini_tools_true", "gemini_followup_false", "gemini_followup_true", "rejected"):
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
            print("CLEANUP: removed this run's containers, temporary SQLite volume and internal network", flush=True)
    passed("temporary resources cleaned up")
    summary = "## Isolated image smoke passed\n\n" + "\n".join("- " + item for item in checks)
    summary += "\n\nSynthetic upstream only; SQLite only; token encoders disabled for offline startup. No real models, Office generation, production data, registry push or deployment.\n"
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
    run(parser.parse_args())
