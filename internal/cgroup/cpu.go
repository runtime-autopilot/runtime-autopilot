package cgroup

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	cgroupV1CPUQuota  = "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"
	cgroupV1CPUPeriod = "/sys/fs/cgroup/cpu/cpu.cfs_period_us"
	cgroupV2CPUMax    = "/sys/fs/cgroup/cpu.max"
)

func CPUEffective(readFile ReadFileFunc) (*float64, error) {
	if readFile == nil {
		readFile = os.ReadFile
	}

	// try cgroup v2
	cpus, err := readCPUV2(readFile)
	if err == nil {
		return cpus, nil
	}

	// fallback to cgroup v1
	return readCPUV1(readFile)
}

func readCPUV2(readFile ReadFileFunc) (*float64, error) {
	data, err := readFile(cgroupV2CPUMax)
	if err != nil {
		return nil, fmt.Errorf("reading cgroup v2 cpu.max: %w", err)
	}

	// "quota period" or "max period" formatting
	parts := strings.Fields(strings.TrimSpace(string(data)))
	if len(parts) != 2 {
		return nil, fmt.Errorf("unexpected format in cpu.max: %q", string(data))
	}

	// unlimited quota
	if parts[0] == "max" {
		return nil, nil
	}

	quota, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return nil, fmt.Errorf("parsing cpu.max quota %q: %w", parts[0], err)
	}

	period, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return nil, fmt.Errorf("parsing cpu.max period %q: %w", parts[1], err)
	}

	if period == 0 {
		return nil, fmt.Errorf("cpu.max period is zero")
	}

	cpus := quota / period

	return &cpus, nil
}

func readCPUV1(readFile ReadFileFunc) (*float64, error) {
	quotaData, err := readFile(cgroupV1CPUQuota)
	if err != nil {
		return nil, fmt.Errorf("reading cgroup v1 cpu.cfs_quota_us: %w", err)
	}

	quotaStr := strings.TrimSpace(string(quotaData))
	quota, err := strconv.ParseFloat(quotaStr, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing cgroup v1 cpu.cfs_quota_us %q: %w", quotaStr, err)
	}

	// negative -> no quota
	if quota < 0 {
		return nil, nil
	}

	periodData, err := readFile(cgroupV1CPUPeriod)
	if err != nil {
		return nil, fmt.Errorf("reading cgroup v1 cpu.cfs_period_us: %w", err)
	}

	periodStr := strings.TrimSpace(string(periodData))
	period, err := strconv.ParseFloat(periodStr, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing cgroup v1 cpu.cfs_period_us %q: %w", periodStr, err)
	}

	if period == 0 {
		return nil, fmt.Errorf("cgroup v1 cpu.cfs_period_us is zero")
	}

	cpus := quota / period
	return &cpus, nil
}
