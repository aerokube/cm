package selenoid

import (
	. "github.com/aandryashin/matchers"
	"path/filepath"
	"testing"
)

func TestGetConfigDir(t *testing.T) {
	selenoidConfigDir := GetSelenoidConfigDir()
	AssertThat(t, selenoidConfigDir, Not{""})
	AssertThat(t, filepath.IsAbs(selenoidConfigDir), Is{true})
	selenoidUIConfigDir := GetSelenoidUIConfigDir()
	AssertThat(t, selenoidUIConfigDir, Not{""})
	AssertThat(t, filepath.IsAbs(selenoidUIConfigDir), Is{true})
}
