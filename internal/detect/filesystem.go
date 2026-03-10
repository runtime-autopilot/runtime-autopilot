package detect

import "os"

type ReadFileFunc func(name string) ([]byte, error)
type StatFunc func(name string) (os.FileInfo, error)

// write access to os.TempDir() and  binary's current working directory
func WritablePaths(tmpDir, workDir string) (isReadOnly bool, writable []string) {
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}

	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			workDir = "."
		}
	}
	candidates := []string{tmpDir, workDir}
	for _, path := range candidates {
		if isPathWritable(path) {
			writable = append(writable, path)
		}
	}

	return len(writable) == 0, writable
}

// create a temporary file in path to confirm write access without relying on permission
func isPathWritable(path string) bool {
	f, err := os.CreateTemp(path, ".autopilot-probe-*")
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
	return true
}
