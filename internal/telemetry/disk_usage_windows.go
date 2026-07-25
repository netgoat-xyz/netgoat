//go:build windows

package telemetry

// Windows does not expose Statfs_t. Keep telemetry available there rather than
// coupling this optional feature to a platform-specific system call; the
// capacity fields remain zero when no portable filesystem statistic is known.
func diskUsageMB(string) (int64, int64) {
	return 0, 0
}
