//go:build !linux

package plugin

import (
	"fmt"
	"runtime"
)

// LoadSO loads a Go plugin from a .so file.
// This is only available on Linux. On other platforms, it returns an error.
func LoadSO(path string) (Plugin, error) {
	return nil, fmt.Errorf("Go plugin (.so) loading is not supported on %s; only Linux", runtime.GOOS)
}
