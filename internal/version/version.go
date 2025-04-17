package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is set during build
	Version = "dev" // TODO: set this to the actual version
	// BuildTime is set during build
	BuildTime = "unknown" // TODO: set this to the actual build time
)

// Info returns version information
func Info() string {
	return fmt.Sprintf("GiraffeCloud CLI %s (built: %s, %s/%s)",
		Version, BuildTime, runtime.GOOS, runtime.GOARCH)
}