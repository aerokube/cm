package selenoid

import (
	assert "github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
)

func TestGetConfigDir(t *testing.T) {
	selenoidConfigDir := GetSelenoidConfigDir()
	assert.NotEmpty(t, selenoidConfigDir)
	assert.True(t, filepath.IsAbs(selenoidConfigDir))
	selenoidUIConfigDir := GetSelenoidUIConfigDir()
	assert.NotEmpty(t, selenoidUIConfigDir)
	assert.True(t, filepath.IsAbs(selenoidUIConfigDir))
}
