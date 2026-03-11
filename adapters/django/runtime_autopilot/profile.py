"""Typed RuntimeProfile dataclass for the Django runtime-autopilot adapter."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class RuntimeProfile:
    """Mirrors the Go sidecar's RuntimeProfile JSON schema (snake_case)."""

    mem_bytes: int | None
    cpu_effective: float | None
    root_read_only: bool
    writable_paths: list[str]
    platform: str
    role: str

    @classmethod
    def from_dict(cls, data: dict) -> "RuntimeProfile":
        """Construct a RuntimeProfile from a parsed JSON dict."""
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
        """Returns ``'tiny'`` (<256 MiB), ``'medium'`` (<1 GiB), or ``'large'``."""
        mb = self.mem_mb()
        if mb is None or mb < 256:
            return "tiny"
        if mb < 1024:
            return "medium"
        return "large"

    def is_read_only(self) -> bool:
        """True when the container root filesystem is read-only."""
        return self.root_read_only
