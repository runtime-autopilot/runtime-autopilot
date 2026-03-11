"""Django runtime-autopilot adapter.

Add ``runtime_autopilot`` to ``INSTALLED_APPS`` to enable automatic
detection and configuration on startup.

Public API::

    from runtime_autopilot import profile

    # After app startup, profile holds the detected RuntimeProfile (or None).
    if profile and profile.root_read_only:
        ...
"""

from __future__ import annotations

from runtime_autopilot.apps import RuntimeAutopilotConfig

# Module-level singleton set by RuntimeAutopilotConfig.ready().
profile: "RuntimeProfile | None" = None  # noqa: F821 (forward ref for docs)

default_app_config = "runtime_autopilot.RuntimeAutopilotConfig"

__all__ = ["RuntimeAutopilotConfig", "profile"]
