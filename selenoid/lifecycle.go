package selenoid

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"io"
)

type LifecycleConfig struct {
	Quiet     bool
	Force     bool
	ConfigDir string
	Browsers  string
	Download  bool

	// Docker specific
	LastVersions int
	RegistryUrl  string
	Tmpfs        int

	// Drivers specific
	BrowsersJsonUrl string
	GithubBaseUrl   string
	OS              string
	Arch            string
	Version         string
}

type Lifecycle struct {
	Logger
	Forceable
	Config       *LifecycleConfig
	downloadable Downloadable
	configurable Configurable
	runnable     Runnable
	closer       io.Closer
}

func NewLifecycle(config *LifecycleConfig) (*Lifecycle, error) {
	lc := Lifecycle{
		Logger:    Logger{Quiet: config.Quiet},
		Forceable: Forceable{Force: config.Force},
		Config:    config,
	}
	if isDockerAvailable() {
		lc.Printf("Using Docker...\n")
		dockerCfg, err := NewDockerConfigurator(config)
		if err != nil {
			return nil, err
		}
		lc.downloadable = dockerCfg
		lc.configurable = dockerCfg
		lc.runnable = dockerCfg
		lc.closer = dockerCfg
	} else {
		lc.Printf("Docker is not supported - using binaries...\n")
		driversCfg := NewDriversConfigurator(config)
		lc.downloadable = driversCfg
		lc.configurable = driversCfg
		lc.runnable = driversCfg
		lc.closer = driversCfg
	}
	return &lc, nil
}

func (l *Lifecycle) Close() {
	if l.closer != nil {
		l.closer.Close()
	}
}

func (l *Lifecycle) Download() error {
	if l.downloadable.IsDownloaded() && !l.Force {
		l.Printf("Selenoid is already downloaded")
		return nil
	} else {
		l.Printf("Downloading Selenoid...")
		_, err := l.downloadable.Download()
		return err
	}
}

func (l *Lifecycle) Configure() error {
	return chain([]func() error{
		func() error {
			return l.Download()
		},
		func() error {
			if l.configurable.IsConfigured() && !l.Force {
				l.Printf("Selenoid is already configured")
				return nil
			}
			l.Printf("Configuring Selenoid...\n")
			_, err := l.configurable.Configure()
			if err == nil {
				l.Printf("Successfully saved configuration to %s\n", getSelenoidConfigPath(l.Config.ConfigDir))
			}
			return err
		},
	})
}

func (l *Lifecycle) Start() error {
	return chain([]func() error{
		func() error {
			return l.Configure()
		},
		func() error {
			if l.runnable.IsRunning() {
				if l.Force {
					l.Printf("Stopping previous Selenoid process...\n")
					err := l.Stop()
					if err != nil {
						return fmt.Errorf("failed to stop previous Selenoid process: %v\n", err)
					}
				} else {
					l.Printf("Selenoid is already running\n")
				}
				return nil
			}
			l.Printf("Starting Selenoid...\n")
			err := l.runnable.Start()
			if err == nil {
				l.Printf("Successfully started Selenoid\n")
			}
			return err
		},
	})
}

func (l *Lifecycle) Stop() error {
	if !l.runnable.IsRunning() {
		l.Printf("Selenoid is not running\n")
		return nil
	}
	l.Printf("Stopping Selenoid...\n")
	err :=  l.runnable.Stop()
	if err == nil {
		l.Printf("Successfully stopped Selenoid\n")
	}
	return err
}

func isDockerAvailable() bool {
	cl, err := client.NewEnvClient()
	if err != nil {
		return false
	}
	_, err = cl.Ping(context.Background())
	return err == nil
}

func chain(steps []func() error) error {
	for _, step := range steps {
		err := step()
		if err != nil {
			return err
		}
	}
	return nil
}
