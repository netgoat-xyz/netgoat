"""Bounded, line-delimited JSON protocol helpers for NetGoat AI workers."""

import json
import sys


MAX_REQUEST_BYTES = 8 * 1024


def iter_requests(stream=None, max_bytes=MAX_REQUEST_BYTES):
    """Yield decoded requests and a protocol error, draining oversized lines."""
    if stream is None:
        stream = sys.stdin.buffer

    while True:
        raw = stream.readline(max_bytes + 2)
        if not raw:
            return

        if len(raw) == max_bytes + 2 and not raw.endswith(b"\n"):
            while raw and not raw.endswith(b"\n"):
                raw = stream.readline(max_bytes + 2)
            yield None, f"request exceeds {max_bytes} bytes"
            continue

        if raw.endswith(b"\n"):
            raw = raw[:-1]
            if raw.endswith(b"\r"):
                raw = raw[:-1]

        if len(raw) > max_bytes:
            yield None, f"request exceeds {max_bytes} bytes"
            continue

        try:
            yield raw.decode("utf-8").strip(), None
        except UnicodeDecodeError:
            yield None, "request is not valid UTF-8"


def write_response(payload, stream=None):
    """Write exactly one JSON response and flush it immediately."""
    if stream is None:
        stream = sys.stdout
    try:
        encoded = json.dumps(
            payload,
            allow_nan=False,
            separators=(",", ":"),
        )
    except (TypeError, ValueError) as exc:
        encoded = json.dumps(
            {"error": f"response serialization failed: {exc}"},
            separators=(",", ":"),
        )
    stream.write(encoded + "\n")
    stream.flush()
