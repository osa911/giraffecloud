//go:build !linux && !darwin

package service

import "os"

// applyPostInstallAttributes is a no-op on non-Unix platforms
func (u *UpdaterService) applyPostInstallAttributes(originalInfo os.FileInfo) error {
	return nil
}
