package selenoid

import (
	"testing"
	. "github.com/aandryashin/matchers"
	"path/filepath"
)

func TestGetConfigDir(t *testing.T) {
	selenoidConfigDir := GetSelenoidConfigDir()
	AssertThat(t, selenoidConfigDir, Not{""})
	AssertThat(t, filepath.IsAbs(selenoidConfigDir), Is{true})
	selenoidUIConfigDir := GetSelenoidUIConfigDir()
	AssertThat(t, selenoidUIConfigDir, Not{""})
	AssertThat(t, filepath.IsAbs(selenoidUIConfigDir), Is{true})
}
