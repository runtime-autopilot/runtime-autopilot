package probe

import (
	"os"

	"github.com/runtime-autopilot/runtime-autopilot/internal/cgroup"
	"github.com/runtime-autopilot/runtime-autopilot/internal/detect"
	"github.com/runtime-autopilot/runtime-autopilot/internal/profile"
)

type Detector interface {
	Detect(profile profile.RuntimeProfile) profile.RuntimeProfile
}

type Pipeline struct {
	detectors []Detector
}

func NewPipeline(detectors ...Detector) Pipeline {
	detector := make([]Detector, len(detectors))
	copy(detector, detectors)

	return Pipeline{detectors: detector}
}

func (p Pipeline) Run() profile.RuntimeProfile {
	prof := profile.RuntimeProfile{}
	for _, detector := range p.detectors {
		prof = detector.Detect(prof)
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

func (detector MemoryDetector) Detect(profile profile.RuntimeProfile) profile.RuntimeProfile {
	limit, _ := cgroup.MemoryLimitBytes(detector.ReadFile) //nolint:errcheck
	profile.MemBytes = limit

	return profile
}

type CPUDetector struct {
	ReadFile cgroup.ReadFileFunc
}

func (detector CPUDetector) Detect(profile profile.RuntimeProfile) profile.RuntimeProfile {
	cpus, _ := cgroup.CPUEffective(detector.ReadFile) //nolint:errcheck 
	profile.CPUEffective = cpus

	return profile
}

type PlatformDetector struct {
	ReadFile detect.ReadFileFunc
	Stat     detect.StatFunc
}

func (detector PlatformDetector) Detect(platform profile.RuntimeProfile) profile.RuntimeProfile {
	platform.Platform = detect.Platform(detector.ReadFile, detector.Stat)
	return platform
}

type RoleDetector struct{}

func (detector RoleDetector) Detect(profile profile.RuntimeProfile) profile.RuntimeProfile {
	profile.Role = detect.Role(os.Args)
	return profile
}

type FilesystemDetector struct {
	TmpDir  string
	WorkDir string
}

func (detector FilesystemDetector) Detect(profile profile.RuntimeProfile) profile.RuntimeProfile {
	readOnly, writable := detect.WritablePaths(detector.TmpDir, detector.WorkDir)
	profile.RootReadOnly = readOnly
	profile.WritablePaths = writable
	return profile
}
