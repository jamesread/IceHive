package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionIsSet(t *testing.T) {
	assert.NotEmpty(t, Version)
	assert.Equal(t, "IceHive", ProjectName)
}
