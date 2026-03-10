package cgroup

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeReader(files map[string]string) ReadFileFunc {
	return func(name string) ([]byte, error) {
		content, ok := files[name]
		if !ok {
			return nil, errors.New("file not found: " + name)
		}
		return []byte(content), nil
	}
}

func TestMemoryLimitBytes(t *testing.T) {
	t.Parallel()

	var (
		mb256 int64 = 256 * 1024 * 1024
		mb512 int64 = 512 * 1024 * 1024
	)

	tests := []struct {
		name    string
		files   map[string]string
		want    *int64
		wantErr bool
	}{
		{
			name:  "cgroup v2 explicit limit",
			files: map[string]string{cgroupV2MemoryMax: "268435456\n"},
			want:  &mb256,
		},
		{
			name:  "cgroup v2 max (unlimited)",
			files: map[string]string{cgroupV2MemoryMax: "max\n"},
			want:  nil,
		},
		{
			name: "cgroup v2 sentinel (>9TiB)",
			files: map[string]string{
				cgroupV2MemoryMax: "9999999999999999\n",
			},
			want: nil,
		},
		{
			name: "cgroup v1 fallback",
			files: map[string]string{
				cgroupV1MemoryLimit: "536870912\n",
			},
			want: &mb512,
		},
		{
			name: "cgroup v1 sentinel",
			files: map[string]string{
				cgroupV1MemoryLimit: "9999999999999999\n",
			},
			want: nil,
		},
		{
			name:    "no cgroup files at all",
			files:   map[string]string{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "cgroup v2 bad value",
			files: map[string]string{
				cgroupV2MemoryMax: "not-a-number\n",
			},
			wantErr: true,
		},
		{
			name: "cgroup v1 bad value",
			files: map[string]string{
				cgroupV1MemoryLimit: "not-a-number\n",
			},
			wantErr: true,
		},
		{
			name: "zero byte file v2",
			files: map[string]string{
				cgroupV2MemoryMax: "",
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := MemoryLimitBytes(makeReader(tc.files))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, *tc.want, *got)
			}
		})
	}
}

func TestCPUEffective(t *testing.T) {
	t.Parallel()

	cpu2 := float64(2)
	cpu15 := float64(1.5)

	tests := []struct {
		name    string
		files   map[string]string
		want    *float64
		wantErr bool
	}{
		{
			name:  "cgroup v2 quota 2 CPUs",
			files: map[string]string{cgroupV2CPUMax: "200000 100000\n"},
			want:  &cpu2,
		},
		{
			name:  "cgroup v2 max (unlimited)",
			files: map[string]string{cgroupV2CPUMax: "max 100000\n"},
			want:  nil,
		},
		{
			name: "cgroup v1 fallback 1.5 CPUs",
			files: map[string]string{
				cgroupV1CPUQuota:  "150000\n",
				cgroupV1CPUPeriod: "100000\n",
			},
			want: &cpu15,
		},
		{
			name: "cgroup v1 no quota (-1)",
			files: map[string]string{
				cgroupV1CPUQuota:  "-1\n",
				cgroupV1CPUPeriod: "100000\n",
			},
			want: nil,
		},
		{
			name:    "no cgroup files at all",
			files:   map[string]string{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "cgroup v2 bad format",
			files: map[string]string{
				cgroupV2CPUMax: "only-one-field\n",
			},
			wantErr: true,
		},
		{
			name: "cgroup v2 zero period",
			files: map[string]string{
				cgroupV2CPUMax: "100000 0\n",
			},
			wantErr: true,
		},
		{
			name: "zero byte file v2",
			files: map[string]string{
				cgroupV2CPUMax: "",
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := CPUEffective(makeReader(tc.files))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.InDelta(t, *tc.want, *got, 0.0001)
			}
		})
	}
}
