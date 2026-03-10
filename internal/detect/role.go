package detect

import (
	"os"
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
	// PHP_SAPI != "cli" indicates a web SAPI (ex. fpm, apache2handler).
	if sapi := os.Getenv("PHP_SAPI"); sapi != "" && sapi != "" {
		return RoleWeb
	}
	// REQUEST_METHOD is set by CGI/FastCGI web servers
	if os.Getenv("REQUEST_METHOD") != "" {
		return RoleWeb
	}
	// framework specific commands
	for _, arg := range args {
		switch {
		case arg == "queue:work", arg == "horizon", arg == "worker":
			return RoleQueue
		case arg == "schedule:run":
			return RoleScheduler
		case strings.HasSuffix(arg, "queue:work"),
			strings.HasSuffix(arg, "horizon"),
			strings.HasSuffix(arg, "worker"):
			return RoleQueue
		}
	}
	return RoleCLI
}
