"""FastAPI lifespan middleware and dependency for runtime-autopilot.

Detects the RuntimeProfile at application startup and stores it in
``app.state.runtime_profile``.  Provides ``get_runtime_profile()`` and
``get_worker_hint()`` as FastAPI dependencies for injection into route handlers.

Auto-configuration applied at startup (honouring ``AUTOPILOT_DRY_RUN``):

- Read-only root FS  warns via structured log (no file-based logging in ASGI apps)
- ``cpu_effective`` set + role ``web``    stores worker hint (max(2, cpu*2))
- ``cpu_effective`` set + role ``queue``  stores concurrency hint (max(1, cpu))
"""

from __future__ import annotations

import json
import logging
import os
import subprocess
import urllib.request
from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager
from dataclasses import dataclass
from typing import Optional

from fastapi import FastAPI, Request

logger = logging.getLogger(__name__)

_DEFAULT_BINARY = "runtime-autopilot"


# ---------------------------------------------------------------------------
# Profile dataclass (same schema as Django / Go)
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class RuntimeProfile:
    """Mirrors the Go sidecar RuntimeProfile JSON schema (snake_case)."""

    mem_bytes: int | None
    cpu_effective: float | None
    root_read_only: bool
    writable_paths: list[str]
    platform: str
    role: str

    @classmethod
    def from_dict(cls, data: dict) -> "RuntimeProfile":
        """Construct from a parsed JSON dict."""
        return cls(
            mem_bytes=data.get("mem_bytes"),
            cpu_effective=data.get("cpu_effective"),
            root_read_only=bool(data.get("root_read_only", False)),
            writable_paths=list(data.get("writable_paths") or []),
            platform=str(data.get("platform", "bare-metal")),
            role=str(data.get("role", "cli")),
        )

    def mem_mb(self) -> float | None:
        """Memory limit in mebibytes, or None when unlimited/unknown."""
        if self.mem_bytes is None:
            return None
        return self.mem_bytes / (1024 * 1024)

    def size_class(self) -> str:
        """Returns 'tiny' (<256 MiB), 'medium' (<1 GiB), or 'large'."""
        mb = self.mem_mb()
        if mb is None or mb < 256:
            return "tiny"
        if mb < 1024:
            return "medium"
        return "large"

    def is_read_only(self) -> bool:
        return self.root_read_only


# ---------------------------------------------------------------------------
# Detection helpers
# ---------------------------------------------------------------------------


def _fetch_json() -> str:
    url = os.environ.get("RUNTIME_AUTOPILOT_URL", "").strip()
    if url:
        return _fetch_http(url)
    return _fetch_binary()


def _fetch_http(url: str) -> str:
    with urllib.request.urlopen(url, timeout=2) as resp:  # noqa: S310
        return resp.read().decode()


def _fetch_binary() -> str:
    binary = (
        os.environ.get("RUNTIME_AUTOPILOT_BIN", _DEFAULT_BINARY).strip()
        or _DEFAULT_BINARY
    )
    result = subprocess.run(  # noqa: S603
        [binary],
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


def detect() -> Optional[RuntimeProfile]:
    """Detect the RuntimeProfile; returns None (with a warning) on failure."""
    try:
        raw_json = _fetch_json()
        data = json.loads(raw_json)
        return RuntimeProfile.from_dict(data)
    except Exception as exc:  # noqa: BLE001
        logger.warning(
            "runtime-autopilot: unable to detect profile — %s. Skipping.",
            exc,
        )
        return None


# ---------------------------------------------------------------------------
# Worker hint computation
# ---------------------------------------------------------------------------


def _compute_worker_hint(profile: RuntimeProfile) -> int | None:
    """Return a recommended worker/concurrency count, or None if not applicable.

    - role ``web``    max(2, floor(cpu_effective * 2))  (Gunicorn / Uvicorn workers)
    - role ``queue``  max(1, floor(cpu_effective))       (Celery concurrency)
    """
    if profile.cpu_effective is None:
        return None
    if profile.role == "web":
        return max(2, int(profile.cpu_effective * 2))
    if profile.role == "queue":
        return max(1, int(profile.cpu_effective))
    return None


# ---------------------------------------------------------------------------
# FastAPI lifespan & dependencies
# ---------------------------------------------------------------------------


@asynccontextmanager
async def autopilot_lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Async context manager for use as a FastAPI lifespan handler.

    Detects the profile synchronously at startup (detection is fast I/O),
    stores it in ``app.state.runtime_profile``, applies auto-configuration,
    and respects AUTOPILOT_DISABLE / AUTOPILOT_DRY_RUN.
    """
    disabled = os.environ.get("AUTOPILOT_DISABLE", "").lower() in ("1", "true", "yes")
    dry_run = os.environ.get("AUTOPILOT_DRY_RUN", "").lower() in ("1", "true", "yes")

    if disabled:
        app.state.runtime_profile = None
        app.state.worker_hint = None
        logger.info("runtime-autopilot: AUTOPILOT_DISABLE is set — skipping.")
    else:
        profile = detect()
        app.state.runtime_profile = profile
        app.state.worker_hint = None

        if profile is not None:
            suffix = " (dry-run)" if dry_run else ""
            logger.info(
                "runtime-autopilot: detected platform=%s role=%s size_class=%s%s",
                profile.platform,
                profile.role,
                profile.size_class(),
                suffix,
            )

            # Read-only filesystem warning.
            if profile.is_read_only():
                logger.warning(
                    "runtime-autopilot: root filesystem is read-only — "
                    "ensure logs go to stdout/stderr, not files%s",
                    suffix,
                )

            # Worker / concurrency hint.
            hint = _compute_worker_hint(profile)
            if hint is not None:
                role_label = (
                    "web (Gunicorn/Uvicorn workers)"
                    if profile.role == "web"
                    else "queue (Celery concurrency)"
                )
                logger.info(
                    "runtime-autopilot: %s role with %.2f CPUs: worker_hint=%d%s",
                    role_label,
                    profile.cpu_effective,
                    hint,
                    suffix,
                )
                if not dry_run:
                    app.state.worker_hint = hint

    yield


def get_runtime_profile(request: Request) -> Optional[RuntimeProfile]:
    """FastAPI dependency: injects the detected RuntimeProfile into route handlers.

    Returns None if detection failed or AUTOPILOT_DISABLE is set.
    """
    return getattr(request.app.state, "runtime_profile", None)


def get_worker_hint(request: Request) -> Optional[int]:
    """FastAPI dependency: injects the computed worker/concurrency hint."""
    return getattr(request.app.state, "worker_hint", None)
