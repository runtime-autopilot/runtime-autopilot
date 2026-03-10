package detect

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlatform(t *testing.T) {
	noFile := func(_ string) ([]byte, error) { return nil, errors.New("not found") }
	noStat := func(_ string) (os.FileInfo, error) { return nil, errors.New("not found") }

	tests := []struct {
		name     string
		envs     map[string]string
		readFile ReadFileFunc
		stat     StatFunc
		want     string
	}{
		{
			name:     "kubernetes via env",
			envs:     map[string]string{"KUBERNETES_SERVICE_HOST": "10.0.0.1"},
			readFile: noFile, stat: noStat,
			want: PlatformKubernetes,
		},
		{
			name:     "kubernetes via secrets path",
			envs:     map[string]string{},
			readFile: noFile,
			stat: func(name string) (os.FileInfo, error) {
				if name == kubernetesSecretsPath {
					return nil, nil
				}
				return nil, errors.New("not found")
			},
			want: PlatformKubernetes,
		},
		{
			name:     "ecs via ECS_CONTAINER_METADATA_URI",
			envs:     map[string]string{"ECS_CONTAINER_METADATA_URI": "http://169.254.170.2/v2/metadata"},
			readFile: noFile, stat: noStat,
			want: PlatformECS,
		},
		{
			name:     "ecs via ECS_CONTAINER_METADATA_URI_V4",
			envs:     map[string]string{"ECS_CONTAINER_METADATA_URI_V4": "http://169.254.170.2/v4/metadata"},
			readFile: noFile, stat: noStat,
			want: PlatformECS,
		},
		{
			name:     "container via .dockerenv",
			envs:     map[string]string{},
			readFile: noFile,
			stat: func(name string) (os.FileInfo, error) {
				if name == dockerEnvPath {
					return nil, nil
				}
				return nil, errors.New("not found")
			},
			want: PlatformContainer,
		},
		{
			name: "container via /proc/1/cgroup",
			envs: map[string]string{},
			readFile: func(name string) ([]byte, error) {
				if name == procCgroupPath {
					return []byte("12:cpuset:/docker/abc123\n"), nil
				}
				return nil, errors.New("not found")
			},
			stat: noStat,
			want: PlatformContainer,
		},
		{
			name:     "bare-metal",
			envs:     map[string]string{},
			readFile: noFile, stat: noStat,
			want: PlatformBareMetal,
		},
	}

	for _, testCase := range tests {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envs {
				t.Setenv(k, v)
			}
			got := Platform(tc.readFile, tc.stat)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRole(t *testing.T) {
	tests := []struct {
		name string
		args []string
		envs map[string]string
		want string
	}{
		{
			name: "web via PHP_SAPI",
			envs: map[string]string{"PHP_SAPI": "fpm-fcgi"},
			args: []string{"php"},
			want: RoleWeb,
		},
		{
			name: "not web when PHP_SAPI=cli",
			envs: map[string]string{"PHP_SAPI": "cli"},
			args: []string{"php", "artisan"},
			want: RoleCLI,
		},
		{
			name: "web via REQUEST_METHOD",
			envs: map[string]string{"REQUEST_METHOD": "GET"},
			args: []string{},
			want: RoleWeb,
		},
		{
			name: "queue via queue:work",
			args: []string{"artisan", "queue:work", "--daemon"},
			want: RoleQueue,
		},
		{
			name: "queue via horizon",
			args: []string{"artisan", "horizon"},
			want: RoleQueue,
		},
		{
			name: "queue via worker",
			args: []string{"python", "worker"},
			want: RoleQueue,
		},
		{
			name: "scheduler via schedule:run",
			args: []string{"artisan", "schedule:run"},
			want: RoleScheduler,
		},
		{
			name: "cli default",
			args: []string{"artisan", "migrate"},
			want: RoleCLI,
		},
	}

	for _, testCase := range tests {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envs {
				t.Setenv(k, v)
			}
			got := Role(tc.args)
			require.Equal(t, tc.want, got)
		})
	}
}

// --- WritablePaths tests ---

func TestWritablePaths(t *testing.T) {
	t.Parallel()

	t.Run("both writable", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		work := t.TempDir()
		readOnly, paths := WritablePaths(tmp, work)
		assert.False(t, readOnly)
		assert.Contains(t, paths, tmp)
		assert.Contains(t, paths, work)
	})

	t.Run("non-existent dir is not writable", func(t *testing.T) {
		t.Parallel()
		readOnly, paths := WritablePaths("/nonexistent-path-xyz", t.TempDir())
		assert.False(t, readOnly) // still has the writable work dir
		assert.Len(t, paths, 1)
	})
}
