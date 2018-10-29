package argotunnel

import (
	"sync"
)

var versionConfig = struct {
	version    string
	setVersion sync.Once
}{
	version: "UNKNOWN",
}

// SetVersion configures the version used by all tunnels
func SetVersion(version string) {
	versionConfig.setVersion.Do(func() {
		versionConfig.version = version
	})
}
