package probe

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runtime-autopilot/runtime-autopilot/internal/cgroup"
	"github.com/runtime-autopilot/runtime-autopilot/internal/detect"
	"github.com/runtime-autopilot/runtime-autopilot/internal/profile"
)

func noFile(_ string) ([]byte, error)      { return nil, errors.New("not found") }
func noStat(_ string) (os.FileInfo, error) { return nil, errors.New("not found") }

func makeMemReader(limitBytes string) cgroup.ReadFileFunc {
	return func(name string) ([]byte, error) {
		if name == "/sys/fs/cgroup/memory.max" {
			return []byte(limitBytes), nil
		}
		return nil, errors.New("not found")
	}
}

func makeCPUReader(quota string) cgroup.ReadFileFunc {
	return func(name string) ([]byte, error) {
		if name == "/sys/fs/cgroup/cpu.max" {
			return []byte(quota), nil
		}
		return nil, errors.New("not found")
	}
}

func TestMemoryDetector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantNil bool
		wantVal int64
	}{
		{name: "256 MiB limit", content: "268435456\n", wantVal: 268435456},
		{name: "unlimited (max)", content: "max\n", wantNil: true},
	}

	for _, testCase := range tests {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := MemoryDetector{ReadFile: makeMemReader(tc.content)}
			p := d.Detect(profile.RuntimeProfile{})
			if tc.wantNil {
				assert.Nil(t, p.MemBytes)
			} else {
				require.NotNil(t, p.MemBytes)
				assert.Equal(t, tc.wantVal, *p.MemBytes)
			}
		})
	}
}

func TestCPUDetector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantNil bool
		wantVal float64
	}{
		{name: "2 CPU quota", content: "200000 100000\n", wantVal: 2.0},
		{name: "unlimited (max)", content: "max 100000\n", wantNil: true},
	}

	for _, testCase := range tests {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := CPUDetector{ReadFile: makeCPUReader(tc.content)}
			p := d.Detect(profile.RuntimeProfile{})
			if tc.wantNil {
				assert.Nil(t, p.CPUEffective)
			} else {
				require.NotNil(t, p.CPUEffective)
				assert.InDelta(t, tc.wantVal, *p.CPUEffective, 0.0001)
			}
		})
	}
}

func TestPlatformDetector(t *testing.T) {
	t.Run("kubernetes via env", func(t *testing.T) {
		t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
		d := PlatformDetector{ReadFile: noFile, Stat: noStat}
		p := d.Detect(profile.RuntimeProfile{})
		assert.Equal(t, detect.PlatformKubernetes, p.Platform)
	})

	t.Run("bare-metal", func(t *testing.T) {
		d := PlatformDetector{ReadFile: noFile, Stat: noStat}
		p := d.Detect(profile.RuntimeProfile{})
		assert.Equal(t, detect.PlatformBareMetal, p.Platform)
	})
}

func TestPipeline(t *testing.T) {
	t.Parallel()

	pipeline := NewPipeline(
		MemoryDetector{ReadFile: makeMemReader("536870912\n")},
		CPUDetector{ReadFile: makeCPUReader("200000 100000\n")},
		PlatformDetector{ReadFile: noFile, Stat: noStat},
		RoleDetector{},
		FilesystemDetector{TmpDir: t.TempDir(), WorkDir: t.TempDir()},
	)

	p := pipeline.Run()

	require.NotNil(t, p.MemBytes)
	assert.Equal(t, int64(536870912), *p.MemBytes)
	require.NotNil(t, p.CPUEffective)
	assert.InDelta(t, 2.0, *p.CPUEffective, 0.0001)
	assert.NotEmpty(t, p.Platform)
	assert.NotEmpty(t, p.Role)
	assert.False(t, p.RootReadOnly)
}

func TestFilesystemDetector(t *testing.T) {
	t.Parallel()

	t.Run("both dirs writable", func(t *testing.T) {
		t.Parallel()
		d := FilesystemDetector{TmpDir: t.TempDir(), WorkDir: t.TempDir()}
		p := d.Detect(profile.RuntimeProfile{})
		assert.False(t, p.RootReadOnly)
		assert.Len(t, p.WritablePaths, 2)
	})

	t.Run("non-writable dirs mark read-only", func(t *testing.T) {
		t.Parallel()
		d := FilesystemDetector{TmpDir: "/nonexistent-xyz", WorkDir: "/nonexistent-abc"}
		p := d.Detect(profile.RuntimeProfile{})
		assert.True(t, p.RootReadOnly)
		assert.Empty(t, p.WritablePaths)
	})
}
