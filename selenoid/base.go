package selenoid

import (
	"log"
	"os"
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
