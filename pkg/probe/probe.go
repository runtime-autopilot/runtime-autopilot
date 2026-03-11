package probe

import (
	"os"

	"github.com/runtime-autopilot/runtime-autopilot/internal/cgroup"
	"github.com/runtime-autopilot/runtime-autopilot/internal/detect"
	"github.com/runtime-autopilot/runtime-autopilot/internal/profile"
)

type Detector interface {
	Detect(p profile.RuntimeProfile) profile.RuntimeProfile
}

type Pipeline struct {
	detectors []Detector
}

func NewPipeline(detectors ...Detector) Pipeline {
	d := make([]Detector, len(detectors))
	copy(d, detectors)
	return Pipeline{detectors: d}
}

func (p Pipeline) Run() profile.RuntimeProfile {
	prof := profile.RuntimeProfile{}
	for _, d := range p.detectors {
		prof = d.Detect(prof)
	}
	return prof
}

var DefaultPipeline = NewPipeline(
	MemoryDetector{},
	CPUDetector{},
	PlatformDetector{},
	RoleDetector{},
	FilesystemDetector{},
)

func Detect() profile.RuntimeProfile {
	return DefaultPipeline.Run()
}

type MemoryDetector struct {
	ReadFile cgroup.ReadFileFunc
}

func (d MemoryDetector) Detect(p profile.RuntimeProfile) profile.RuntimeProfile {
	limit, _ := cgroup.MemoryLimitBytes(d.ReadFile) //nolint:errcheck
	p.MemBytes = limit
	return p
}

type CPUDetector struct {
	ReadFile cgroup.ReadFileFunc
}

func (d CPUDetector) Detect(p profile.RuntimeProfile) profile.RuntimeProfile {
	cpus, _ := cgroup.CPUEffective(d.ReadFile) //nolint:errcheck
	p.CPUEffective = cpus
	return p
}

type PlatformDetector struct {
	ReadFile detect.ReadFileFunc
	Stat     detect.StatFunc
}

func (d PlatformDetector) Detect(p profile.RuntimeProfile) profile.RuntimeProfile {
	p.Platform = detect.Platform(d.ReadFile, d.Stat)
	return p
}

type RoleDetector struct{}

func (d RoleDetector) Detect(p profile.RuntimeProfile) profile.RuntimeProfile {
	p.Role = detect.Role(os.Args)
	return p
}

type FilesystemDetector struct {
	TmpDir  string
	WorkDir string
}

func (d FilesystemDetector) Detect(p profile.RuntimeProfile) profile.RuntimeProfile {
	readOnly, writable := detect.WritablePaths(d.TmpDir, d.WorkDir)
	p.RootReadOnly = readOnly
	p.WritablePaths = writable
	return p
}
