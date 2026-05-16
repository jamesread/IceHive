package buildinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultsAreNonEmpty(t *testing.T) {
	assert.NotEmpty(t, Version)
	assert.NotEmpty(t, Commit)
	assert.NotEmpty(t, Date)
}
