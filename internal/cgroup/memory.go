// Package cgroup provides helpers for reading Linux cgroup v1/v2 resource limits.
package cgroup

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	cgroupV1MemoryLimit = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	cgroupV2MemoryMax   = "/sys/fs/cgroup/memory.max"
	// memory limit is considered "unlimited" (9 * 1024^4 bytes = 9 TiB).
	unlimitedThreshold = 9 * 1024 * 1024 * 1024 * 1024
)

type ReadFileFunc func(name string) ([]byte, error)

// reads the effective memory limit from cgroup v2 then v1.
func MemoryLimitBytes(readFile ReadFileFunc) (*int64, error) {
	if readFile == nil {
		readFile = os.ReadFile
	}

	// try cgroup v2
	if limit, err := readMemoryV2(readFile); err == nil {
		return limit, err
	}
	// fallback cgroup v1
	return readMemoryV1(readFile)
}

func readMemoryV2(readFile ReadFileFunc) (*int64, error) {
	data, err := readFile(cgroupV2MemoryMax)
	if err != nil {
		return nil, fmt.Errorf("reading cgroup v2 memory.max: %w", err)
	}

	val := strings.TrimSpace(string(data))
	// if unlimited
	if val == "max" {
		return nil, nil
	}

	limit, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing cgroup v2 memory.max %q: %w", val, err)
	}

	if limit >= unlimitedThreshold {
		return nil, nil
	}

	return &limit, nil
}
func readMemoryV1(readFile ReadFileFunc) (*int64, error) {
	data, err := readFile(cgroupV1MemoryLimit)
	if err != nil {
		return nil, fmt.Errorf("reading cgroup v1 memory.limit_in_bytes: %w", err)
	}

	val := strings.TrimSpace(string(data))
	limit, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing cgroup v1 memory.limit_in_bytes %q: %w", val, err)
	}

	if limit >= unlimitedThreshold {
		return nil, nil
	}

	return &limit, nil
}
