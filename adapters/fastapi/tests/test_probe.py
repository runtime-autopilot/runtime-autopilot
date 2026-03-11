"""Pytest tests for the FastAPI runtime-autopilot adapter."""

from __future__ import annotations

import json
import subprocess
from contextlib import asynccontextmanager
from typing import Optional
from unittest.mock import MagicMock, patch

import pytest
from fastapi import Depends, FastAPI
from fastapi.testclient import TestClient

from runtime_autopilot_fastapi.probe import (
    RuntimeProfile,
    autopilot_lifespan,
    detect,
    get_runtime_profile,
    get_worker_hint,
)

# ---------------------------------------------------------------------------
# Test fixtures (profile data dicts)
# ---------------------------------------------------------------------------

FIXTURE_K8S_WEB = {
    "mem_bytes": 268435456,
    "cpu_effective": 2.0,
    "root_read_only": False,
    "writable_paths": ["/tmp"],
    "platform": "kubernetes",
    "role": "web",
}

FIXTURE_RO_QUEUE = {
    "mem_bytes": None,
    "cpu_effective": None,
    "root_read_only": True,
    "writable_paths": [],
    "platform": "bare-metal",
    "role": "queue",
}

FIXTURE_ECS_WEB = {
    "mem_bytes": 536870912,
    "cpu_effective": 1.5,
    "root_read_only": False,
    "writable_paths": ["/tmp"],
    "platform": "ecs",
    "role": "web",
}

FIXTURE_ECS_QUEUE = {
    "mem_bytes": 536870912,
    "cpu_effective": 1.5,
    "root_read_only": False,
    "writable_paths": ["/tmp"],
    "platform": "ecs",
    "role": "queue",
}


def _mock_proc(fixture: dict) -> MagicMock:
    """Return a fake subprocess.CompletedProcess that outputs fixture JSON."""
    m = MagicMock(spec=subprocess.CompletedProcess)
    m.returncode = 0
    m.stdout = json.dumps(fixture)
    m.stderr = ""
    return m


def _failed_proc(returncode: int = 127, stderr: str = "command not found") -> MagicMock:
    """Return a fake subprocess.CompletedProcess that simulates a binary failure."""
    m = MagicMock(spec=subprocess.CompletedProcess)
    m.returncode = returncode
    m.stdout = ""
    m.stderr = stderr
    return m


def _make_app() -> FastAPI:
    """Build a minimal FastAPI app wired with the autopilot lifespan and both dependencies."""

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        async with autopilot_lifespan(app):
            yield

    app = FastAPI(lifespan=lifespan)

    @app.get("/profile")
    def profile_route(
        p: Optional[RuntimeProfile] = Depends(get_runtime_profile),
    ) -> dict:
        if p is None:
            return {"platform": None, "role": None, "size_class": None}
        return {"platform": p.platform, "role": p.role, "size_class": p.size_class()}

    @app.get("/worker-hint")
    def worker_hint_route(hint: Optional[int] = Depends(get_worker_hint)) -> dict:
        return {"worker_hint": hint}

    @app.get("/health")
    def health() -> dict:
        return {"ok": True}

    return app


# ---------------------------------------------------------------------------
# RuntimeProfile unit tests
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    "mem_bytes, expected_class",
    [
        (None, "tiny"),
        (128 * 1024 * 1024, "tiny"),
        (256 * 1024 * 1024, "medium"),
        (512 * 1024 * 1024, "medium"),
        (1024 * 1024 * 1024, "large"),
    ],
)
def test_size_class(mem_bytes: Optional[int], expected_class: str) -> None:
    p = RuntimeProfile.from_dict({"mem_bytes": mem_bytes})
    assert p.size_class() == expected_class


@pytest.mark.parametrize(
    "fixture",
    [FIXTURE_K8S_WEB, FIXTURE_RO_QUEUE, FIXTURE_ECS_WEB],
    ids=["kubernetes-web", "ro-queue", "ecs-web"],
)
def test_from_dict_roundtrip(fixture: dict) -> None:
    p = RuntimeProfile.from_dict(fixture)
    assert p.platform == fixture["platform"]
    assert p.role == fixture["role"]
    assert p.root_read_only == fixture["root_read_only"]
    assert p.mem_bytes == fixture["mem_bytes"]


def test_from_dict_defaults() -> None:
    """Empty dict produces safe defaults — no KeyError."""
    p = RuntimeProfile.from_dict({})
    assert p.platform == "bare-metal"
    assert p.role == "cli"
    assert p.root_read_only is False
    assert p.mem_bytes is None
    assert p.cpu_effective is None


def test_mem_mb_none() -> None:
    assert RuntimeProfile.from_dict({}).mem_mb() is None


def test_mem_mb_value() -> None:
    p = RuntimeProfile.from_dict({"mem_bytes": 512 * 1024 * 1024})
    assert abs((p.mem_mb() or 0) - 512.0) < 0.01


def test_is_read_only() -> None:
    assert RuntimeProfile.from_dict({"root_read_only": True}).is_read_only() is True
    assert RuntimeProfile.from_dict({"root_read_only": False}).is_read_only() is False


# ---------------------------------------------------------------------------
# detect() unit tests
# ---------------------------------------------------------------------------


@pytest.mark.parametrize("fixture", [FIXTURE_K8S_WEB, FIXTURE_ECS_WEB])
def test_detect_via_binary(monkeypatch: pytest.MonkeyPatch, fixture: dict) -> None:
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    with patch("subprocess.run", return_value=_mock_proc(fixture)):
        result = detect()
    assert result is not None
    assert result.platform == fixture["platform"]
    assert result.role == fixture["role"]


def test_detect_returns_none_on_binary_failure(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    with patch("subprocess.run", return_value=_failed_proc()):
        result = detect()
    assert result is None


def test_detect_returns_none_on_invalid_json(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    m = MagicMock(spec=subprocess.CompletedProcess)
    m.returncode = 0
    m.stdout = "not-valid-json"
    m.stderr = ""
    with patch("subprocess.run", return_value=m):
        result = detect()
    assert result is None


# ---------------------------------------------------------------------------
# FastAPI lifespan + dependency integration tests
# ---------------------------------------------------------------------------


def test_lifespan_stores_profile_and_exposes_via_dependency(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    monkeypatch.delenv("AUTOPILOT_DISABLE", raising=False)
    monkeypatch.delenv("AUTOPILOT_DRY_RUN", raising=False)

    with patch("subprocess.run", return_value=_mock_proc(FIXTURE_K8S_WEB)):
        app = _make_app()
        with TestClient(app) as client:
            resp = client.get("/profile")
            assert resp.status_code == 200
            data = resp.json()
            assert data["platform"] == "kubernetes"
            assert data["role"] == "web"
            assert data["size_class"] == "medium"


@pytest.mark.parametrize(
    "fixture",
    [FIXTURE_K8S_WEB, FIXTURE_RO_QUEUE, FIXTURE_ECS_WEB],
    ids=["kubernetes-web", "ro-queue", "ecs-web"],
)
def test_lifespan_all_profiles_health_ok(
    monkeypatch: pytest.MonkeyPatch, fixture: dict
) -> None:
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    monkeypatch.delenv("AUTOPILOT_DISABLE", raising=False)
    with patch("subprocess.run", return_value=_mock_proc(fixture)):
        app = _make_app()
        with TestClient(app) as client:
            assert client.get("/health").status_code == 200


def test_lifespan_disabled_returns_null_profile(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("AUTOPILOT_DISABLE", "true")
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)

    app = _make_app()
    with TestClient(app) as client:
        resp = client.get("/profile")
        assert resp.status_code == 200
        assert resp.json()["platform"] is None


def test_lifespan_binary_not_found_returns_null_profile(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    monkeypatch.delenv("AUTOPILOT_DISABLE", raising=False)

    with patch("subprocess.run", return_value=_failed_proc()):
        app = _make_app()
        with TestClient(app) as client:
            resp = client.get("/profile")
            assert resp.status_code == 200
            assert resp.json()["platform"] is None


# ---------------------------------------------------------------------------
# Worker hint tests
# ---------------------------------------------------------------------------


def test_worker_hint_web_role(monkeypatch: pytest.MonkeyPatch) -> None:
    """Web role with cpu_effective=2.0  hint = max(2, floor(2.0 * 2)) = 4."""
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    monkeypatch.delenv("AUTOPILOT_DISABLE", raising=False)
    monkeypatch.delenv("AUTOPILOT_DRY_RUN", raising=False)

    with patch("subprocess.run", return_value=_mock_proc(FIXTURE_K8S_WEB)):
        app = _make_app()
        with TestClient(app) as client:
            resp = client.get("/worker-hint")
            assert resp.status_code == 200
            assert resp.json()["worker_hint"] == 4  # max(2, int(2.0 * 2))


def test_worker_hint_queue_role(monkeypatch: pytest.MonkeyPatch) -> None:
    """Queue role with cpu_effective=1.5  hint = max(1, floor(1.5)) = 1."""
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    monkeypatch.delenv("AUTOPILOT_DISABLE", raising=False)
    monkeypatch.delenv("AUTOPILOT_DRY_RUN", raising=False)

    with patch("subprocess.run", return_value=_mock_proc(FIXTURE_ECS_QUEUE)):
        app = _make_app()
        with TestClient(app) as client:
            resp = client.get("/worker-hint")
            assert resp.status_code == 200
            assert resp.json()["worker_hint"] == 1  # max(1, int(1.5))


def test_worker_hint_none_when_no_cpu(monkeypatch: pytest.MonkeyPatch) -> None:
    """No cpu_effective  worker_hint is None."""
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    monkeypatch.delenv("AUTOPILOT_DISABLE", raising=False)
    monkeypatch.delenv("AUTOPILOT_DRY_RUN", raising=False)

    with patch("subprocess.run", return_value=_mock_proc(FIXTURE_RO_QUEUE)):
        app = _make_app()
        with TestClient(app) as client:
            resp = client.get("/worker-hint")
            assert resp.status_code == 200
            assert resp.json()["worker_hint"] is None


def test_worker_hint_none_when_disabled(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("AUTOPILOT_DISABLE", "true")
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)

    app = _make_app()
    with TestClient(app) as client:
        resp = client.get("/worker-hint")
        assert resp.status_code == 200
        assert resp.json()["worker_hint"] is None


def test_dry_run_suppresses_worker_hint(monkeypatch: pytest.MonkeyPatch) -> None:
    """AUTOPILOT_DRY_RUN=true  worker_hint logged but not stored (stays None)."""
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    monkeypatch.delenv("AUTOPILOT_DISABLE", raising=False)
    monkeypatch.setenv("AUTOPILOT_DRY_RUN", "true")

    with patch("subprocess.run", return_value=_mock_proc(FIXTURE_K8S_WEB)):
        app = _make_app()
        with TestClient(app) as client:
            resp = client.get("/worker-hint")
            assert resp.status_code == 200
            assert resp.json()["worker_hint"] is None
