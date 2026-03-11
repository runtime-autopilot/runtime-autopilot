"""FastAPI runtime-autopilot adapter.

Usage::

    from contextlib import asynccontextmanager
    from fastapi import FastAPI
    from runtime_autopilot_fastapi import autopilot_lifespan, get_runtime_profile

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        async with autopilot_lifespan(app):
            yield

    app = FastAPI(lifespan=lifespan)

    @app.get("/info")
    def info(profile = Depends(get_runtime_profile)):
        return {"platform": profile.platform if profile else None}
"""

from runtime_autopilot_fastapi.probe import (
    autopilot_lifespan,
    get_runtime_profile,
)

__all__ = ["autopilot_lifespan", "get_runtime_profile"]
