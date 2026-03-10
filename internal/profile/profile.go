package profile

import (
	"encoding/json"
)

// keep detected runtime environment configuration
type RuntimeProfile struct {
	// container memory limit in bytes, or null if unlimited/unknown
	MemBytes *int64 `json:"-"`
	//effective CPU count derived from cgroup quota
	CPUEffective *float64 `json:"-"`
	// true when neither os.TempDir nor the working directory is writable
	RootReadOnly  bool     `json:"-"`
	WritablePaths []string `json:"-"`
	// "kubernetes", "ecs", "container", or "bare-metal"
	Platform string `json:"-"`
	// "web", "queue", "scheduler", or "cli"
	Role string `json:"-"`
}

// JSON-serialisable form of RuntimeProfile with snake_case keys
type snakeProfile struct {
	MemBytes      *int64   `json:"mem_bytes"`
	CPUEffective  *float64 `json:"cpu_effective"`
	RootReadOnly  bool     `json:"root_read_only"`
	WritablePaths []string `json:"writable_paths"`
	Platform      string   `json:"platform"`
	Role          string   `json:"role"`
}

// JSON object with snake_case keys
func (p RuntimeProfile) MarshalJSON() ([]byte, error) {
	paths := p.WritablePaths
	if paths == nil {
		paths = []string{}
	}

	return json.Marshal(snakeProfile{
		MemBytes:      p.MemBytes,
		CPUEffective:  p.CPUEffective,
		RootReadOnly:  p.RootReadOnly,
		WritablePaths: paths,
		Platform:      p.Platform,
		Role:          p.Role,
	})
}

// decodes a snake_case JSON object into RuntimeProfile
func (p *RuntimeProfile) UnmarshalJSON(data []byte) error {
	var snakeProfile snakeProfile
	if err := json.Unmarshal(data, &snakeProfile); err != nil {
		return err
	}
	p.MemBytes = snakeProfile.MemBytes
	p.CPUEffective = snakeProfile.CPUEffective
	p.RootReadOnly = snakeProfile.RootReadOnly
	p.WritablePaths = snakeProfile.WritablePaths
	p.Platform = snakeProfile.Platform
	p.Role = snakeProfile.Role

	return nil
}

//	container size tier.
//
// "tiny"   - memory limit < 256 MiB (or unknown)
// "medium" - memory limit < 1024 MiB
// "large"  - memory limit >= 1024 MiB
func (p RuntimeProfile) SizeClass() string {
	if p.MemBytes == nil {
		return "tiny"
	}
	mb := float64(*p.MemBytes) / (1024 * 1024)
	switch {
	case mb < 256:
		return "tiny"
	case mb < 1024:
		return "medium"
	default:
		return "large"
	}
}

func (p RuntimeProfile) MemMB() *float64 {
	if p.MemBytes == nil {
		return nil
	}
	mb := float64(*p.MemBytes) / (1024 * 1024)
	return &mb
}
