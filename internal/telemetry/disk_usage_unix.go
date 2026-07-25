//go:build !windows

package telemetry

import "syscall"

// diskUsageMB preserves filesystem capacity reporting on Unix-like targets.
func diskUsageMB(path string) (int64, int64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	blockSize := int64(stat.Bsize)
	return (int64(stat.Blocks) * blockSize) / (1024 * 1024),
		(int64(stat.Bavail) * blockSize) / (1024 * 1024)
}
