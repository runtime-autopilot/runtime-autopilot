"""Binary/HTTP probe for the Django runtime-autopilot adapter.

Resolution order:
  1. HTTP GET $RUNTIME_AUTOPILOT_URL (if set)
  2. Shell subprocess calling the runtime-autopilot binary
     (RUNTIME_AUTOPILOT_BIN env, defaults to 'runtime-autopilot' on $PATH)
"""

from __future__ import annotations

import json
import logging
import os
import subprocess
import urllib.request
from typing import Optional

from runtime_autopilot.profile import RuntimeProfile

logger = logging.getLogger(__name__)

_DEFAULT_BINARY = "runtime-autopilot"

# Module-level cache — populated after the first successful detect() call.
_cached_profile: RuntimeProfile | None = None
_cache_populated: bool = False


def detect(*, force: bool = False) -> Optional[RuntimeProfile]:
    """Detect the RuntimeProfile.

    Results are cached after the first successful call.  Pass ``force=True``
    to re-run detection (useful in tests).

    Returns ``None`` and logs a warning if detection fails.
    """
    global _cached_profile, _cache_populated  # noqa: PLW0603

    if _cache_populated and not force:
        return _cached_profile

    try:
        raw_json = _fetch_json()
        data = json.loads(raw_json)
        profile = RuntimeProfile.from_dict(data)
    except Exception as exc:  # noqa: BLE001
        logger.warning(
            "runtime-autopilot: unable to detect profile — %s. Skipping auto-configuration.",
            exc,
        )
        _cached_profile = None
        _cache_populated = True
        return None

    _cached_profile = profile
    _cache_populated = True
    return profile


def _fetch_json() -> str:
    url = os.environ.get("RUNTIME_AUTOPILOT_URL", "").strip()
    if url:
        return _fetch_http(url)
    return _fetch_binary()


def _fetch_http(url: str) -> str:
    with urllib.request.urlopen(url, timeout=2) as resp:  # noqa: S310
        return resp.read().decode()


def _fetch_binary() -> str:
    binary = os.environ.get("RUNTIME_AUTOPILOT_BIN", _DEFAULT_BINARY).strip() or _DEFAULT_BINARY
    result = subprocess.run(  # noqa: S603
        [binary, "--json"],
        capture_output=True,
        text=True,
        timeout=5,
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"runtime-autopilot exited {result.returncode}: {result.stderr.strip()}"
        )
    stdout = result.stdout.strip()
    if not stdout:
        raise RuntimeError("runtime-autopilot produced no output")
    return stdout
