package argotunnel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetVersion(t *testing.T) {
	version := versionConfig.version
	versions := []string{
		"x.y.z",
		"a.b.c",
		"1.2.3",
	}

	for _, v := range versions {
		SetVersion(v)
	}

	assert.Equalf(t, versions[0], versionConfig.version, "test version matches first set")
	assert.NotEqualf(t, versionConfig.version, version, "test version does not match default")
}
