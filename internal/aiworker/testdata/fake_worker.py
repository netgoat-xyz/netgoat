#!/usr/bin/env python3
"""Tiny line worker used to exercise process lifecycle edge cases."""

import json
import os
from pathlib import Path
import sys
import time


def first_attempt(marker):
    path = Path(marker)
    try:
        path.touch(exist_ok=False)
        return True
    except FileExistsError:
        return False


for raw_line in sys.stdin:
    request = raw_line.strip()

    if request == "hang":
        time.sleep(30)
        continue

    if request.startswith("hang-ready:"):
        Path(request.removeprefix("hang-ready:")).touch()
        time.sleep(30)
        continue

    if request.startswith("crash-once:"):
        marker = request.removeprefix("crash-once:")
        if first_attempt(marker):
            os._exit(7)
        response = {"label": "restarted", "score": 0.75}
    elif request.startswith("bad-type-once:"):
        marker = request.removeprefix("bad-type-once:")
        if first_attempt(marker):
            response = {"label": "stale", "score": "not-a-number"}
        else:
            response = {"score": 0.75}
    elif request == "large":
        response = {"label": "x" * 4096, "score": 0.75}
    else:
        response = {"label": request, "score": 0.75}

    # Intentionally omit flush=True. The Go worker must launch Python in
    # unbuffered mode for this response to arrive before its deadline.
    print(json.dumps(response))
