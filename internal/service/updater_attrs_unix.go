//go:build linux || darwin

package service

import (
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

// applyPostInstallAttributes preserves owner/group and best-effort labels on Unix platforms
func (u *UpdaterService) applyPostInstallAttributes(originalInfo os.FileInfo) error {
	if runtime.GOOS != "windows" {
		if stat, ok := originalInfo.Sys().(*syscall.Stat_t); ok {
			_ = os.Chown(u.currentExePath, int(stat.Uid), int(stat.Gid))
		}
		if runtime.GOOS == "linux" {
			if _, err := exec.LookPath("restorecon"); err == nil {
				_ = exec.Command("restorecon", "-F", u.currentExePath).Run()
			}
		}
		if runtime.GOOS == "darwin" {
			if _, err := exec.LookPath("xattr"); err == nil {
				_ = exec.Command("xattr", "-d", "com.apple.quarantine", u.currentExePath).Run()
			}
		}
	}
	return nil
}
