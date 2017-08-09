package selenoid

import (
	"log"
	"os"
	"os/user"
	"path/filepath"
)

type StatusAware interface {
	Status()
	UIStatus()
}

type Downloadable interface {
	IsDownloaded() bool
	Download() (string, error)
	IsUIDownloaded() bool
	DownloadUI() (string, error)
}

type Configurable interface {
	IsConfigured() bool
	Configure() (*SelenoidConfig, error)
}

type Runnable interface {
	IsRunning() bool
	Start() error
	Stop() error
	IsUIRunning() bool
	StartUI() error
	StopUI() error
}

type Logger struct {
	Quiet bool
}

func (c *Logger) Printf(format string, v ...interface{}) {
	if !c.Quiet {
		log.Printf(format, v...)
	}
}

type ConfigDirAware struct {
	ConfigDir string
}

func (c *ConfigDirAware) createConfigDir() error {
	err := os.MkdirAll(c.ConfigDir, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

type Forceable struct {
	Force bool
}

type VersionAware struct {
	Version string
}

type DownloadAware struct {
	DownloadNeeded bool
}

type RequestedBrowsersAware struct {
	Browsers string
}

type ArgsAware struct {
	Args string
}

type EnvAware struct {
	Env string
}

type BrowserEnvAware struct {
	BrowserEnv string
}

func getHomeDir() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return usr.HomeDir
}

func joinPaths(baseDir string, elem []string) string {
	p := filepath.Join(append([]string{baseDir}, elem...)...)
	ap, _ := filepath.Abs(p)
	return ap
}

var (
	selenoidConfigDirElem   = []string{".aerokube", "selenoid"}
	selenoidUIConfigDirElem = []string{".aerokube", "selenoid-ui"}
)

func GetSelenoidConfigDir() string {
	return joinPaths(getHomeDir(), selenoidConfigDirElem)
}

func GetSelenoidUIConfigDir() string {
	return joinPaths(getHomeDir(), selenoidUIConfigDirElem)
}
