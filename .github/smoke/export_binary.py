"""Package an already verified CI program; never execute it or read runtime data."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil


MAX_BINARY_BYTES = 256 * 1024 * 1024
BUILD_INFO = re.compile(
    r"CANDIDATE_BUILD_INFO Go=(go1\.26\.9) main=one-api GOOS=linux "
    r"GOARCH=(amd64|arm64) CGO_ENABLED=1 SHA256=([0-9a-f]{64})"
)


def export(binary, build_info, source_sha, image_id, output):
    binary, build_info, output = map(Path, (binary, build_info, output))
    if not re.fullmatch(r"[0-9a-f]{40}", source_sha):
        raise ValueError("source SHA must identify an exact commit")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", image_id):
        raise ValueError("image identity must be a local immutable image ID")
    if binary.is_symlink() or not binary.is_file():
        raise ValueError("candidate must be a regular file")
    if not 0 < binary.stat().st_size <= MAX_BINARY_BYTES:
        raise ValueError("candidate exceeds the binary evidence size bound")
    if build_info.is_symlink() or build_info.stat().st_size > 4096:
        raise ValueError("build identity record exceeds its bound")
    identity = BUILD_INFO.fullmatch(build_info.read_text().strip())
    if identity is None:
        raise ValueError("missing or unexpected verified build identity")
    output.mkdir(mode=0o700, exist_ok=False)
    copied = output / "one-api"
    try:
        # Bound the copy itself, including a candidate that grows after stat().
        with binary.open("rb") as source, copied.open("xb") as destination:
            remaining = MAX_BINARY_BYTES
            while remaining:
                chunk = source.read(min(1024 * 1024, remaining))
                if not chunk:
                    break
                destination.write(chunk)
                remaining -= len(chunk)
            if source.read(1):
                raise ValueError("candidate exceeds the binary evidence size bound")
        copied.chmod(0o600)
        with copied.open("rb") as stream:
            digest = hashlib.file_digest(stream, "sha256").hexdigest()
        if digest != identity[3]:
            raise ValueError("program bytes differ from the verified CI program")
        record = {
            "schema_version": "onehub.binary-evidence/v1",
            "source_sha": source_sha,
            "image_id": image_id,
            "binary": "one-api",
            "binary_sha256": digest,
            "go_version": identity[1],
            "goos": "linux",
            "goarch": identity[2],
            "cgo_enabled": "1",
            "main": "one-api",
        }
        manifest = output / "manifest.json"
        manifest.write_text(json.dumps(record, indent=2) + "\n")
        manifest.chmod(0o600)
        return record
    except Exception:
        # Only this invocation's newly created directory belongs to us.
        shutil.rmtree(output)
        raise


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for option in ("binary", "build-info", "source-sha", "image-id", "output"):
        parser.add_argument("--" + option, required=True)
    args = parser.parse_args()
    export(args.binary, args.build_info, args.source_sha, args.image_id, args.output)
