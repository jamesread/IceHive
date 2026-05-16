package common

import (
	"testing"

	"github.com/icehive/icehive/services/common/pkg/buildinfo"
	"github.com/stretchr/testify/assert"
)

func TestProjectNameAndBuildinfo(t *testing.T) {
	assert.Equal(t, "IceHive", ProjectName)
	assert.NotEmpty(t, buildinfo.Version)
}
