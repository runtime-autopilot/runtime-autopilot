package detect

import (
	"os"
	"strings"
)

const (
	// kubernetes pod
	PlatformKubernetes = "kubernetes"
	// AWS ECS/Fargate
	PlatformECS = "ecs"
	// generic container
	PlatformContainer = "container"
	// no container environment is detected
	PlatformBareMetal = "bare-metal"

	kubernetesSecretsPath = "/var/run/secrets/kubernetes.io"
	dockerEnvPath         = "/.dockerenv"
	procCgroupPath        = "/proc/1/cgroup"
)

// detect runtime platform using env variables and filesystem markers
func Platform(readFile ReadFileFunc, stat StatFunc) string {
	if readFile == nil {
		readFile = os.ReadFile
	}
	if stat == nil {
		stat = os.Stat
	}
	// k8s service host env var/secrets mount
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return PlatformKubernetes
	}
	if _, err := stat(kubernetesSecretsPath); err == nil {
		return PlatformKubernetes
	}
	// ECS/Fargate metadata URI
	if os.Getenv("ECS_CONTAINER_METADATA_URI") != "" ||
		os.Getenv("ECS_CONTAINER_METADATA_URI_V4") != "" {
		return PlatformECS
	}
	// generic containers ex. docker
	if _, err := stat(dockerEnvPath); err == nil {
		return PlatformContainer
	}
	if data, err := readFile(procCgroupPath); err == nil {
		if strings.Contains(string(data), "docker") {
			return PlatformContainer
		}
	}

	return PlatformBareMetal
}
