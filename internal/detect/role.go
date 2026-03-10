package detect

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	//HTTP requests.
	RoleWeb = "web"
	// background queue/worker processes.
	RoleQueue = "queue"
	// cron-style scheduler processes.
	RoleScheduler = "scheduler"
	// unrecognized processes.
	RoleCLI = "cli"
)

// inspect env variables and detect type
func Role(args []string) string {
	// PHP

	// PHP_SAPI != "cli" indicates a web SAPI (ex. fpm, apache2handler).
	if sapi := os.Getenv("PHP_SAPI"); sapi != "" && sapi != "cli" {
		return RoleWeb
	}
	// REQUEST_METHOD is set by CGI/FastCGI web servers
	if os.Getenv("REQUEST_METHOD") != "" {
		return RoleWeb
	}

	// Python / WSGI / ASG
	// Celery
	if role, ok := detectCelery(args); ok {
		return role
	}
	for _, arg := range args {
		base := filepath.Base(arg)
		switch {
		// WSGI / ASGI web servers
		case base == "gunicorn", base == "uvicorn":
			return RoleWeb
		// Django dev server: python manage.py runserver.
		case (base == "manage.py" || base == "manage") && containsArg(args, "runserver"):
			return RoleWeb
		}
	}

	// Laravel/artisan
	for _, arg := range args {
		switch {
		case arg == "queue:work", arg == "horizon":
			return RoleQueue
		case arg == "schedule:run":
			return RoleScheduler
		case strings.HasSuffix(arg, "queue:work"),
			strings.HasSuffix(arg, "horizon"):
			return RoleQueue
		}
	}
	return RoleCLI
}

// detect celery binary entry-point
func detectCelery(args []string) (role string, found bool) {
	for i, arg := range args {
		if filepath.Base(arg) != "celery" {
			continue
		}
		// Celery found -> inspect the rest for the sub-command.
		for _, sub := range args[i+1:] {
			switch sub {
			case "worker":
				return RoleQueue, true
			case "beat":
				return RoleScheduler, true
			}
		}
		// Celery present but no recognised sub-command -> treat as cli.
		return RoleCLI, true
	}
	return "", false
}

func containsArg(args []string, target string) bool {
	for _, a := range args {
		if a == target {
			return true
		}
	}
	return false
}
