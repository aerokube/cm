package selenoid

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/fatih/color"
	"io"
)

type LifecycleConfig struct {
	Quiet       bool
	Force       bool
	ConfigDir   string
	Browsers    string
	BrowserEnv  string
	Download    bool
	Args        string
	Env         string
	Version     string
	Port        int
	DisableLogs bool

	// Docker specific
	LastVersions int
	RegistryUrl  string
	ShmSize      int
	Tmpfs        int
	VNC          bool
	UserNS       string

	// Drivers specific
	UseDrivers      bool
	BrowsersJsonUrl string
	GithubBaseUrl   string
	OS              string
	Arch            string
}

type Lifecycle struct {
	Logger
	Forceable
	Config       *LifecycleConfig
	argsAware    ArgsProvider
	statusAware  StatusProvider
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
	if config.UseDrivers {
		lc.Titlef("Using driver binaries...")
		driversCfg := NewDriversConfigurator(config)
		lc.argsAware = driversCfg
		lc.statusAware = driversCfg
		lc.downloadable = driversCfg
		lc.configurable = driversCfg
		lc.runnable = driversCfg
		lc.closer = driversCfg
		return &lc, nil
	}
	if !isDockerAvailable() {
		return nil, errors.New("can not access Docker: make sure you have Docker installed and current user has access permissions")
	}
	lc.Titlef("Using %v", color.BlueString("Docker"))
	dockerCfg, err := NewDockerConfigurator(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Docker support: %v", err)
	}
	lc.argsAware = dockerCfg
	lc.statusAware = dockerCfg
	lc.downloadable = dockerCfg
	lc.configurable = dockerCfg
	lc.runnable = dockerCfg
	lc.closer = dockerCfg
	return &lc, nil
}

func (l *Lifecycle) Close() {
	if l.closer != nil {
		l.closer.Close()
	}
}

func (l *Lifecycle) Status() {
	l.statusAware.Status()
}

func (l *Lifecycle) UIStatus() {
	l.statusAware.UIStatus()
}

func (l *Lifecycle) Download() error {
	if l.downloadable.IsDownloaded() && !l.Force {
		l.Titlef("Selenoid is already downloaded")
		return nil
	} else {
		l.Titlef("Downloading Selenoid...")
		_, err := l.downloadable.Download()
		return err
	}
}

func (l *Lifecycle) DownloadUI() error {
	if l.downloadable.IsUIDownloaded() && !l.Force {
		l.Titlef("Selenoid UI is already downloaded")
		return nil
	} else {
		l.Titlef("Downloading Selenoid UI...")
		_, err := l.downloadable.DownloadUI()
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
				l.Titlef("Selenoid is already configured")
				return nil
			}
			l.Titlef("Configuring Selenoid...")
			_, err := l.configurable.Configure()
			if err == nil {
				l.Titlef("Configuration saved to %v", color.GreenString(getSelenoidConfigPath(l.Config.ConfigDir)))
			}
			return err
		},
	})
}

func (l *Lifecycle) PrintArgs() error {
	return chain([]func() error{
		func() error {
			return l.Download()
		},
		func() error {
			l.Titlef("Printing Selenoid args...")
			return l.argsAware.PrintArgs()
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
					l.Titlef("Stopping previous Selenoid instance...")
					err := l.Stop()
					if err != nil {
						return fmt.Errorf("failed to stop previous Selenoid instance: %v", err)
					}
				} else {
					l.Titlef("Selenoid is already running")
					return nil
				}
			}

			l.Titlef("Starting Selenoid...")
			err := l.runnable.Start()
			if err == nil {
				l.Titlef("Successfully started Selenoid")
			}
			return err
		},
	})
}

func (l *Lifecycle) PrintUIArgs() error {
	return chain([]func() error{
		func() error {
			return l.DownloadUI()
		},
		func() error {
			l.Titlef("Printing Selenoid UI args...")
			return l.argsAware.PrintUIArgs()
		},
	})
}

func (l *Lifecycle) StartUI() error {
	return chain([]func() error{
		func() error {
			return l.DownloadUI()
		},
		func() error {
			if l.runnable.IsUIRunning() {
				if l.Force {
					l.Titlef("Stopping previous Selenoid UI instance...")
					err := l.StopUI()
					if err != nil {
						return fmt.Errorf("failed to stop previous Selenoid UI instance: %v", err)
					}
				} else {
					l.Titlef("Selenoid UI is already running")
					return nil
				}
			}
			l.Titlef("Starting Selenoid UI...")
			err := l.runnable.StartUI()
			if err == nil {
				l.Titlef("Successfully started Selenoid UI")
			}
			return err
		},
	})
}

func (l *Lifecycle) Stop() error {
	if !l.runnable.IsRunning() {
		l.Titlef("Selenoid is not running")
		return nil
	}
	l.Titlef("Stopping Selenoid...")
	err := l.runnable.Stop()
	if err == nil {
		l.Titlef("Successfully stopped Selenoid")
	}
	return err
}

func (l *Lifecycle) StopUI() error {
	if !l.runnable.IsUIRunning() {
		l.Titlef("Selenoid UI is not running")
		return nil
	}
	l.Titlef("Stopping Selenoid UI...")
	err := l.runnable.StopUI()
	if err == nil {
		l.Titlef("Successfully stopped Selenoid UI")
	}
	return err
}

func isDockerAvailable() bool {
	cl, err := client.NewClientWithOpts(client.FromEnv)
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
