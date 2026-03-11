# runtime-autopilot

Detects container resource limits (CPU, memory, platform, process role) and exposes them as structured JSON. Adapters for Django, FastAPI, and Laravel use the binary to auto-configure worker counts, cache sizes, and logging at startup.

## How it works

The binary reads cgroup v2/v1 files, environment variables, and filesystem markers to produce a profile:

```json
{
  "mem_bytes": 536870912,
  "cpu_effective": 1.5,
  "root_read_only": false,
  "writable_paths": ["/tmp"],
  "platform": "kubernetes",
  "role": "web"
}
```

Platforms: `kubernetes`, `ecs`, `container`, `bare-metal`  
Roles: `web` (gunicorn, uvicorn, PHP-FPM), `queue` (celery worker), `scheduler` (celery beat), `cli`

## Build

```bash
make build                        # builds to bin/runtime-autopilot
# or cross-compile for Linux:
GOOS=linux GOARCH=arm64 go build -o bin/runtime-autopilot ./cmd/runtime-autopilot
```

## Run

```bash
./bin/runtime-autopilot           # one-shot JSON to stdout
./bin/runtime-autopilot -pretty   # pretty-printed
./bin/runtime-autopilot -serve :9000  # HTTP server at /profile and /health
```

## Adapters

All adapters look for the binary via `RUNTIME_AUTOPILOT_BIN` (default: `runtime-autopilot` on `$PATH`) or an HTTP sidecar via `RUNTIME_AUTOPILOT_URL`.

| Env var | Default | Purpose |
|---|---|---|
| `RUNTIME_AUTOPILOT_BIN` | `runtime-autopilot` | Path to binary |
| `RUNTIME_AUTOPILOT_URL` | — | HTTP URL when running in server mode |
| `AUTOPILOT_DISABLE` | — | Set to `true` to skip detection entirely |
| `AUTOPILOT_DRY_RUN` | — | Set to `true` to detect but not apply |

### Django

```bash
pip install django-runtime-autopilot
```

Add to `INSTALLED_APPS`:

```python
INSTALLED_APPS = [
    ...
    "runtime_autopilot",
]
```

The adapter reads the profile on startup and is available in `AppConfig.ready()`.

### FastAPI

```bash
pip install fastapi-runtime-autopilot
```

```python
from contextlib import asynccontextmanager
from runtime_autopilot_fastapi.probe import autopilot_lifespan, get_runtime_profile

@asynccontextmanager
async def lifespan(app):
    async with autopilot_lifespan(app):
        yield

app = FastAPI(lifespan=lifespan)

@app.get("/info")
def info(profile = Depends(get_runtime_profile)):
    return {"size": profile.size_class() if profile else "unknown"}
```

### Laravel

```bash
composer require runtime-autopilot/laravel-autopilot
```

The service provider is auto-discovered. Access the profile via the `Probe` facade:

```php
use RuntimeAutopilot\Probe;

$profile = Probe::detect();
$profile->sizeClass();   // "tiny" | "medium" | "large"
$profile->memMb();       
```

## Testing

```bash
# Go tests
make test

# Adapter tests (requires PHP, Python locally)
make adapter-laravel
make adapter-django
make adapter-fastapi

# All adapter tests via Docker 
GOOS=linux GOARCH=arm64 go build -o adapters/django/runtime-autopilot ./cmd/runtime-autopilot
docker build -f adapters/django/Dockerfile.test -t ra-django-test adapters/django && docker run --rm --memory=512m --cpus=1.5 ra-django-test
docker build -f adapters/fastapi/Dockerfile.test -t ra-fastapi-test adapters/fastapi && docker run --rm --memory=512m --cpus=1.5 ra-fastapi-test
docker build -f adapters/laravel/Dockerfile.test -t ra-laravel-test adapters/laravel && docker run --rm --memory=512m --cpus=1.5 ra-laravel-test
```

## Linting

```bash
make lint                     # runs golangci-lint
make fmt                      # runs gofmt + goimports
```

Requires golangci-lint v2.11.3+.
