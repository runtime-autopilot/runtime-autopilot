"""Django AppConfig for runtime_autopilot.

Add ``"runtime_autopilot"`` to ``INSTALLED_APPS``.  The ``ready()`` hook
runs detection and applies safe configuration overrides.
"""

from __future__ import annotations

import logging
import os

from django.apps import AppConfig

logger = logging.getLogger(__name__)


class RuntimeAutopilotConfig(AppConfig):
    name = "runtime_autopilot"
    verbose_name = "Runtime Autopilot"

    def ready(self) -> None:
        if os.environ.get("AUTOPILOT_DISABLE", "").lower() in ("1", "true", "yes"):
            logger.info("runtime-autopilot: AUTOPILOT_DISABLE is set — skipping.")
            return

        dry_run = os.environ.get("AUTOPILOT_DRY_RUN", "").lower() in (
            "1",
            "true",
            "yes",
        )

        from runtime_autopilot import probe
        import runtime_autopilot as _module

        profile = probe.detect()
        _module.profile = profile  # publish module-level singleton

        if profile is None:
            return

        self._apply_logging(profile, dry_run)
        self._apply_cache(profile, dry_run)
        self._apply_workers(profile, dry_run)

    # ------------------------------------------------------------------
    # Private helpers
    # ------------------------------------------------------------------

    def _apply_logging(self, profile: object, dry_run: bool) -> None:  # type: ignore[override]
        from runtime_autopilot.profile import RuntimeProfile

        if not isinstance(profile, RuntimeProfile) or not profile.is_read_only():
            return

        self._decide(
            "root filesystem is read-only: switching LOGGING to StreamHandler(sys.stderr)",
            dry_run,
            self._configure_stream_logging,
        )

    def _apply_cache(self, profile: object, dry_run: bool) -> None:  # type: ignore[override]
        from runtime_autopilot.profile import RuntimeProfile

        if not isinstance(profile, RuntimeProfile) or not profile.is_read_only():
            return

        try:
            import django_redis  # noqa: F401

            has_redis = True
        except ImportError:
            has_redis = False

        if not has_redis:
            return

        self._decide(
            "root filesystem is read-only and django-redis is available: switching CACHES to redis",
            dry_run,
            self._configure_redis_cache,
        )

    def _apply_workers(self, profile: object, dry_run: bool) -> None:  # type: ignore[override]
        """Publish worker-count hints as env vars readable by Gunicorn/Celery configs.

        - Role ``web`` + cpu_effective set  AUTOPILOT_GUNICORN_WORKERS
          Formula: max(2, floor(cpu_effective * 2))

        - Role ``queue`` + cpu_effective set AUTOPILOT_CELERY_CONCURRENCY
          Formula: max(1, floor(cpu_effective))
        """
        from runtime_autopilot.profile import RuntimeProfile

        if not isinstance(profile, RuntimeProfile) or profile.cpu_effective is None:
            return

        if profile.role == "web":
            workers = max(2, int(profile.cpu_effective * 2))
            self._decide(
                f"web role with {profile.cpu_effective:.2f} CPUs: "
                f"setting AUTOPILOT_GUNICORN_WORKERS={workers}",
                dry_run,
                lambda: os.environ.update({"AUTOPILOT_GUNICORN_WORKERS": str(workers)}),
            )

        elif profile.role == "queue":
            concurrency = max(1, int(profile.cpu_effective))
            self._decide(
                f"queue role with {profile.cpu_effective:.2f} CPUs: "
                f"setting AUTOPILOT_CELERY_CONCURRENCY={concurrency}",
                dry_run,
                lambda: os.environ.update(
                    {"AUTOPILOT_CELERY_CONCURRENCY": str(concurrency)}
                ),
            )

    def _configure_stream_logging(self) -> None:
        from django.conf import settings

        settings.LOGGING = {  # type: ignore[assignment]
            "version": 1,
            "disable_existing_loggers": False,
            "handlers": {
                "stderr": {
                    "class": "logging.StreamHandler",
                    "stream": "ext://sys.stderr",
                }
            },
            "root": {"handlers": ["stderr"], "level": "WARNING"},
        }
        import logging.config

        logging.config.dictConfig(settings.LOGGING)

    def _configure_redis_cache(self) -> None:
        from django.conf import settings

        if not hasattr(settings, "CACHES"):
            settings.CACHES = {}  # type: ignore[assignment]

        settings.CACHES["default"] = {  # type: ignore[index]
            "BACKEND": "django_redis.cache.RedisCache",
            "LOCATION": os.environ.get("REDIS_URL", "redis://localhost:6379/0"),
            "OPTIONS": {"CLIENT_CLASS": "django_redis.client.DefaultClient"},
        }

    def _decide(self, message: str, dry_run: bool, action: object) -> None:
        suffix = " (dry-run, skipping)" if dry_run else ""
        logger.info("runtime-autopilot: %s%s", message, suffix)
        if not dry_run and callable(action):
            action()
