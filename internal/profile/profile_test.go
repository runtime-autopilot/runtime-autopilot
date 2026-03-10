package profile

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr64(v int64) *int64       { return &v }
func ptrF64(v float64) *float64 { return &v }

func TestSizeClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		memBytes *int64
		want     string
	}{
		{name: "nil (unknown)", memBytes: nil, want: "tiny"},
		{name: "128 MiB", memBytes: ptr64(128 * 1024 * 1024), want: "tiny"},
		{name: "exactly 256 MiB", memBytes: ptr64(256 * 1024 * 1024), want: "medium"},
		{name: "512 MiB", memBytes: ptr64(512 * 1024 * 1024), want: "medium"},
		{name: "exactly 1024 MiB", memBytes: ptr64(1024 * 1024 * 1024), want: "large"},
		{name: "2 GiB", memBytes: ptr64(2 * 1024 * 1024 * 1024), want: "large"},
	}

	for _, testCase := range tests {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := RuntimeProfile{MemBytes: tc.memBytes}
			assert.Equal(t, tc.want, p.SizeClass())
		})
	}
}

func TestMemMB(t *testing.T) {
	t.Parallel()

	t.Run("nil mem", func(t *testing.T) {
		t.Parallel()
		p := RuntimeProfile{}
		assert.Nil(t, p.MemMB())
	})

	t.Run("512 MiB", func(t *testing.T) {
		t.Parallel()
		p := RuntimeProfile{MemBytes: ptr64(512 * 1024 * 1024)}
		require.NotNil(t, p.MemMB())
		assert.InDelta(t, 512.0, *p.MemMB(), 0.01)
	})
}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	p := RuntimeProfile{
		MemBytes:      ptr64(268435456),
		CPUEffective:  ptrF64(2.0),
		RootReadOnly:  true,
		WritablePaths: []string{"/tmp"},
		Platform:      "kubernetes",
		Role:          "web",
	}

	data, err := json.Marshal(p)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, float64(268435456), m["mem_bytes"])
	assert.Equal(t, float64(2.0), m["cpu_effective"])
	assert.Equal(t, true, m["root_read_only"])
	assert.Equal(t, "kubernetes", m["platform"])
	assert.Equal(t, "web", m["role"])

	assert.NotContains(t, m, "memBytes")
	assert.NotContains(t, m, "cpuEffective")
}

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	raw := `{
		"mem_bytes": 536870912,
		"cpu_effective": 1.5,
		"root_read_only": false,
		"writable_paths": ["/tmp", "/app"],
		"platform": "ecs",
		"role": "queue"
	}`

	var p RuntimeProfile
	require.NoError(t, json.Unmarshal([]byte(raw), &p))

	require.NotNil(t, p.MemBytes)
	assert.Equal(t, int64(536870912), *p.MemBytes)
	require.NotNil(t, p.CPUEffective)
	assert.InDelta(t, 1.5, *p.CPUEffective, 0.0001)
	assert.False(t, p.RootReadOnly)
	assert.Equal(t, []string{"/tmp", "/app"}, p.WritablePaths)
	assert.Equal(t, "ecs", p.Platform)
	assert.Equal(t, "queue", p.Role)
}

func TestMarshalJSON_NullFields(t *testing.T) {
	t.Parallel()

	p := RuntimeProfile{Platform: "bare-metal", Role: "cli"}
	data, err := json.Marshal(p)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Nil(t, m["mem_bytes"])
	assert.Nil(t, m["cpu_effective"])
	assert.Equal(t, []interface{}{}, m["writable_paths"])
}
