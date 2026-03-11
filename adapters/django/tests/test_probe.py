"""Pytest tests for the Django runtime-autopilot adapter."""

from __future__ import annotations

import json
import subprocess
from unittest.mock import MagicMock, patch

import pytest

from runtime_autopilot.profile import RuntimeProfile
from runtime_autopilot import probe as probe_module


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(autouse=True)
def reset_probe_cache():
    """Reset the module-level probe cache between tests."""
    probe_module._cache_populated = False
    probe_module._cached_profile = None
    yield
    probe_module._cache_populated = False
    probe_module._cached_profile = None


FIXTURE_KUBERNETES_WEB = {
    "mem_bytes": 268435456,
    "cpu_effective": 2.0,
    "root_read_only": False,
    "writable_paths": ["/tmp"],
    "platform": "kubernetes",
    "role": "web",
}

FIXTURE_READ_ONLY_QUEUE = {
    "mem_bytes": None,
    "cpu_effective": None,
    "root_read_only": True,
    "writable_paths": [],
    "platform": "bare-metal",
    "role": "queue",
}

FIXTURE_ECS_SCHEDULER = {
    "mem_bytes": 536870912,
    "cpu_effective": 1.5,
    "root_read_only": False,
    "writable_paths": ["/tmp", "/app"],
    "platform": "ecs",
    "role": "scheduler",
}


# ---------------------------------------------------------------------------
# RuntimeProfile tests
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    "data, expected_mem, expected_cpu, expected_ro, expected_platform, expected_role",
    [
        (FIXTURE_KUBERNETES_WEB, 268435456, 2.0, False, "kubernetes", "web"),
        (FIXTURE_READ_ONLY_QUEUE, None, None, True, "bare-metal", "queue"),
        (FIXTURE_ECS_SCHEDULER, 536870912, 1.5, False, "ecs", "scheduler"),
        ({}, None, None, False, "bare-metal", "cli"),
    ],
)
def test_profile_from_dict(
    data, expected_mem, expected_cpu, expected_ro, expected_platform, expected_role
):
    p = RuntimeProfile.from_dict(data)
    assert p.mem_bytes == expected_mem
    assert p.cpu_effective == expected_cpu
    assert p.root_read_only == expected_ro
    assert p.platform == expected_platform
    assert p.role == expected_role


@pytest.mark.parametrize(
    "mem_bytes, expected",
    [
        (None, "tiny"),
        (128 * 1024 * 1024, "tiny"),
        (256 * 1024 * 1024, "medium"),
        (512 * 1024 * 1024, "medium"),
        (1024 * 1024 * 1024, "large"),
        (2 * 1024 * 1024 * 1024, "large"),
    ],
)
def test_size_class(mem_bytes, expected):
    p = RuntimeProfile.from_dict({"mem_bytes": mem_bytes})
    assert p.size_class() == expected


def test_mem_mb_none():
    p = RuntimeProfile.from_dict({})
    assert p.mem_mb() is None


def test_mem_mb_value():
    p = RuntimeProfile.from_dict({"mem_bytes": 512 * 1024 * 1024})
    assert abs(p.mem_mb() - 512.0) < 0.01


# ---------------------------------------------------------------------------
# Probe tests
# ---------------------------------------------------------------------------


def _mock_completed_process(fixture: dict) -> MagicMock:
    mock = MagicMock(spec=subprocess.CompletedProcess)
    mock.returncode = 0
    mock.stdout = json.dumps(fixture)
    mock.stderr = ""
    return mock


@pytest.mark.parametrize("fixture", [FIXTURE_KUBERNETES_WEB, FIXTURE_ECS_SCHEDULER])
def test_probe_detect_via_binary(monkeypatch, fixture):
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    monkeypatch.setenv("RUNTIME_AUTOPILOT_BIN", "runtime-autopilot")

    with patch("subprocess.run", return_value=_mock_completed_process(fixture)):
        result = probe_module.detect(force=True)

    assert result is not None
    assert result.platform == fixture["platform"]
    assert result.role == fixture["role"]


def test_probe_detect_binary_failure(monkeypatch):
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)

    failed = MagicMock(spec=subprocess.CompletedProcess)
    failed.returncode = 1
    failed.stdout = ""
    failed.stderr = "binary not found"

    with patch("subprocess.run", return_value=failed):
        result = probe_module.detect(force=True)

    assert result is None


def test_probe_detect_cached(monkeypatch):
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)

    with patch("subprocess.run", return_value=_mock_completed_process(FIXTURE_KUBERNETES_WEB)) as mock_run:
        probe_module.detect(force=True)
        probe_module.detect()  # should use cache
        assert mock_run.call_count == 1


def test_probe_detect_via_url(monkeypatch):
    monkeypatch.setenv("RUNTIME_AUTOPILOT_URL", "http://localhost:9000/profile")

    with patch("urllib.request.urlopen") as mock_urlopen:
        mock_cm = MagicMock()
        mock_cm.__enter__ = MagicMock(return_value=mock_cm)
        mock_cm.__exit__ = MagicMock(return_value=False)
        mock_cm.read.return_value = json.dumps(FIXTURE_ECS_SCHEDULER).encode()
        mock_urlopen.return_value = mock_cm

        result = probe_module.detect(force=True)

    assert result is not None
    assert result.platform == "ecs"


def test_probe_autopilot_disable(monkeypatch):
    """AUTOPILOT_DISABLE is handled at the AppConfig layer, not probe layer."""
    # probe.detect() itself is always callable; disable is checked in AppConfig.ready()
    monkeypatch.delenv("RUNTIME_AUTOPILOT_URL", raising=False)
    with patch("subprocess.run", return_value=_mock_completed_process(FIXTURE_KUBERNETES_WEB)):
        result = probe_module.detect(force=True)
    assert result is not None
