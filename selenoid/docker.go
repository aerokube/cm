package selenoid

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/go-units"
	"io/ioutil"
	"log"
	"sort"

	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/aerokube/selenoid/config"
	authconfig "github.com/docker/cli/cli/config"
	configtypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/heroku/docker-registry-client/registry"
	"github.com/mattn/go-colorable"

	"net/http"
	"path/filepath"
	"regexp"
	"runtime"

	"encoding/base64"
	"github.com/aerokube/cm/render/rewriter"
	dc "github.com/aerokube/util/docker"
	"github.com/fatih/color"
	. "github.com/fvbommel/sortorder"
	"io"
	"net/url"
)

const (
	semicolon               = ";"
	colon                   = ":"
	Latest                  = "latest"
	firefox                 = "firefox"
	android                 = "android"
	edge                    = "MicrosoftEdge"
	opera                   = "opera"
	tag_1216                = "12.16"
	selenoidImage           = "aerokube/selenoid"
	selenoidUIImage         = "aerokube/selenoid-ui"
	videoRecorderImage      = "selenoid/video-recorder:latest-release"
	selenoidContainerName   = "selenoid"
	ggrUIContainerName      = "ggr-ui"
	selenoidUIContainerName = "selenoid-ui"
	overrideHome            = "OVERRIDE_HOME"
	dockerApiVersion        = "DOCKER_API_VERSION"
)

type SelenoidConfig map[string]config.Versions

type DockerConfigurator struct {
	Logger
	ConfigDirAware
	VersionAware
	DownloadAware
	RequestedBrowsersAware
	ArgsAware
	EnvAware
	BrowserEnvAware
	PortAware
	UserNSAware
	LogsAware
	GracefulAware
	LastVersions int
	Pull         bool
	RegistryUrl  string
	BrowsersJson string
	ShmSize      int
	Tmpfs        int
	VNC          bool
	docker       *client.Client
	reg          *registry.Registry
	authConfig   *configtypes.AuthConfig
	registryHost string
}

func NewDockerConfigurator(config *LifecycleConfig) (*DockerConfigurator, error) {
	c := &DockerConfigurator{
		Logger:                 Logger{Quiet: config.Quiet},
		ConfigDirAware:         ConfigDirAware{ConfigDir: config.ConfigDir},
		VersionAware:           VersionAware{Version: config.Version},
		DownloadAware:          DownloadAware{DownloadNeeded: config.Download},
		RequestedBrowsersAware: RequestedBrowsersAware{Browsers: config.Browsers},
		ArgsAware:              ArgsAware{Args: config.Args},
		EnvAware:               EnvAware{Env: config.Env},
		BrowserEnvAware:        BrowserEnvAware{BrowserEnv: config.BrowserEnv},
		PortAware:              PortAware{Port: config.Port},
		UserNSAware:            UserNSAware{UserNS: config.UserNS},
		LogsAware:              LogsAware{DisableLogs: config.DisableLogs},
		GracefulAware:          GracefulAware{Graceful: config.Graceful, GracefulTimeout: config.GracefulTimeout},
		RegistryUrl:            config.RegistryUrl,
		BrowsersJson:           config.BrowsersJson,
		LastVersions:           config.LastVersions,
		ShmSize:                config.ShmSize,
		Tmpfs:                  config.Tmpfs,
		VNC:                    config.VNC,
	}
	if c.Quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
	err := c.initDockerClient()
	if err != nil {
		return nil, fmt.Errorf("new configurator: %v", err)
	}
	authConfig, err := c.initAuthConfig()
	if err != nil {
		c.Errorf("Failed to load authentication configuration, using default values: %v", err)
	} else {
		c.authConfig = authConfig
	}
	return c, nil
}

func (c *DockerConfigurator) initDockerClient() error {
	docker, err := dc.CreateCompatibleDockerClient(
		func(specifiedApiVersion string) {
			c.Pointf("Using Docker API version: %s", specifiedApiVersion)
		},
		func(determinedApiVersion string) {
			c.Pointf("Your Docker API version is %s", determinedApiVersion)
		},
		func(defaultApiVersion string) {
			c.Pointf("Did not manage to determine your Docker API version - using default version: %s", defaultApiVersion)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to init Docker client: %v", err)
	}
	c.docker = docker
	return nil
}

func (c *DockerConfigurator) initAuthConfig() (*configtypes.AuthConfig, error) {
	configFile, err := authconfig.Load("")
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(c.RegistryUrl)
	if err != nil {
		return nil, err
	}

	registryHost := u.Host
	if c.RegistryUrl != DefaultRegistryUrl {
		c.registryHost = registryHost
	}
	if cfg, ok := configFile.AuthConfigs[registryHost]; ok {
		c.Titlef(`Loaded authentication data for "%s"`, registryHost)
		return &cfg, nil
	}

	return nil, nil
}

func (c *DockerConfigurator) getRegistryClient() *registry.Registry {
	if c.reg != nil {
		return c.reg
	}

	u := strings.TrimSuffix(c.RegistryUrl, "/")
	username, password := "", ""
	if c.authConfig != nil {
		username, password = c.authConfig.Username, c.authConfig.Password
	}
	reg := &registry.Registry{
		URL: u,
		Client: &http.Client{
			Transport: registry.WrapTransport(http.DefaultTransport, u, username, password),
		},
		Logf: func(format string, args ...interface{}) {
			c.Tracef(format, args...)
		},
	}

	if err := reg.Ping(); err != nil {
		c.Errorf("Docker Registry is not available: %v", err)
		return nil
	}

	c.reg = reg
	return reg
}

func (c *DockerConfigurator) Close() error {
	if c.docker != nil {
		return c.docker.Close()
	}
	return nil
}

func (c *DockerConfigurator) Status() {
	selenoidImage := c.getSelenoidImage()
	if selenoidImage != nil {
		c.Pointf("Using Selenoid image: %s (%s)", selenoidImage.RepoTags[0], selenoidImage.ID)
	} else {
		c.Pointf("Selenoid image is not present")
	}
	configPath := getSelenoidConfigPath(c.ConfigDir)
	c.Pointf("Selenoid configuration directory is %s", c.ConfigDir)
	if fileExists(configPath) {
		c.Pointf("Selenoid configuration file is %s", configPath)
	} else {
		c.Pointf("Selenoid is not configured")
	}
	selenoidContainer := c.getSelenoidContainer()
	if selenoidContainer != nil {
		c.Pointf("Selenoid container is running: %s (%s)", selenoidContainerName, selenoidContainer.ID)
	} else {
		c.Pointf("Selenoid container is not running")
	}
}

func (c *DockerConfigurator) UIStatus() {
	selenoidUIImage := c.getSelenoidUIImage()
	if selenoidUIImage != nil {
		c.Pointf("Using Selenoid UI image: %s (%s)", selenoidUIImage.RepoTags[0], selenoidUIImage.ID)
	} else {
		c.Pointf("Selenoid UI image is not present")
	}
	selenoidUIContainer := c.getSelenoidUIContainer()
	if selenoidUIContainer != nil {
		c.Pointf("Selenoid UI container is running: %s (%s)", selenoidUIContainerName, selenoidUIContainer.ID)
	} else {
		c.Pointf("Selenoid UI container is not running")
	}
}

func (c *DockerConfigurator) IsDownloaded() bool {
	return c.getSelenoidImage() != nil
}

func (c *DockerConfigurator) getSelenoidImage() *types.ImageSummary {
	return c.getImage(selenoidImage, c.Version)
}

func (c *DockerConfigurator) IsUIDownloaded() bool {
	return c.getSelenoidUIImage() != nil
}

func (c *DockerConfigurator) getSelenoidUIImage() *types.ImageSummary {
	return c.getImage(selenoidUIImage, c.Version)
}

func (c *DockerConfigurator) getImage(name string, version string) *types.ImageSummary {
	images, err := c.docker.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		c.Errorf("Failed to list images: %v", err)
		return nil
	}
	return findMatchingImage(images, name, version)
}

func findMatchingImage(images []types.ImageSummary, name string, version string) *types.ImageSummary {
	sort.Slice(images, func(i, j int) bool {
		return images[i].Created > images[j].Created
	})
	for _, img := range images {
		const colon = ":"
		for _, tag := range img.RepoTags {
			nameAndVersion := strings.Split(tag, colon)
			if len(nameAndVersion) >= 2 {
				imageVersion := nameAndVersion[len(nameAndVersion)-1]
				imageName := strings.TrimSuffix(tag, colon+imageVersion)
				if strings.HasSuffix(imageName, name) && (version == "" || version == Latest || version == imageVersion) {
					return &img
				}
			}
		}
	}
	return nil
}

func (c *DockerConfigurator) Download() (string, error) {
	return c.downloadImpl(selenoidImage, c.Version, "failed to pull Selenoid image")
}

func (c *DockerConfigurator) DownloadUI() (string, error) {
	return c.downloadImpl(selenoidUIImage, c.Version, "failed to pull Selenoid UI image")
}

func (c *DockerConfigurator) downloadImpl(imageName string, version string, errorMessage string) (string, error) {
	if version == Latest {
		latestVersion := c.getLatestImageVersion(imageName)
		if latestVersion != nil {
			version = *latestVersion
		}
	}
	ref := c.getFullyQualifiedImageRef(imageName)
	if version != Latest {
		ref = imageWithTag(ref, version)
	}
	if !c.pullImage(context.Background(), ref) {
		return "", errors.New(errorMessage)
	}
	return ref, nil
}

func (c *DockerConfigurator) getLatestImageVersion(imageName string) *string {
	tags := c.fetchImageTags(imageName)
	if len(tags) > 0 {
		return &tags[0]
	}
	return nil
}

func (c *DockerConfigurator) IsConfigured() bool {
	return fileExists(getSelenoidConfigPath(c.ConfigDir))
}

func (c *DockerConfigurator) Configure() (*SelenoidConfig, error) {
	err := c.createConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}
	if c.BrowsersJson != "" {
		return c.syncWithConfig()
	}

	cfg := c.createConfig()
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %v", err)
	}
	return &cfg, ioutil.WriteFile(getSelenoidConfigPath(c.ConfigDir), data, 0644)
}

func (c *DockerConfigurator) syncWithConfig() (*SelenoidConfig, error) {
	c.Titlef(`Requested to sync configuration from "%v"...`, color.GreenString(c.BrowsersJson))
	data, err := ioutil.ReadFile(c.BrowsersJson)
	if err != nil {
		return nil, fmt.Errorf("failed to read browsers.json from %s: %v", c.BrowsersJson, err)
	}
	var cfg SelenoidConfig
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse browsers.json from %s: %v", c.BrowsersJson, err)
	}
	if c.DownloadNeeded {
		for _, versions := range cfg {
			for _, version := range versions.Versions {
				if ref, ok := version.Image.(string); ok {
					ctx := context.Background()
					if !c.pullImage(ctx, ref) {
						return nil, fmt.Errorf("failed to pull image %s from browsers.json file %s", ref, c.BrowsersJson)
					}
				} else {
					c.Pointf("Skipping non-Docker image specification: %v", version.Image)
				}
			}
		}
		c.pullVideoRecorderImage()
	}
	return &cfg, ioutil.WriteFile(getSelenoidConfigPath(c.ConfigDir), data, 0644)
}

func (c *DockerConfigurator) createConfig() SelenoidConfig {
	requestedBrowsers := parseRequestedBrowsers(&c.Logger, c.Browsers)
	browsersToIterate := c.getBrowsersToIterate(requestedBrowsers)
	browsers := make(map[string]config.Versions)
	for browserName, image := range browsersToIterate {
		c.Titlef(`Processing browser "%v"...`, color.GreenString(browserName))
		tags := c.fetchImageTags(image)
		if c.VNC {
			c.Pointf("Requested to download VNC images but this feature is now deprecated as all images contain VNC.")
		}
		versionConstraint := requestedBrowsers[browserName]
		pulledTags := c.filterTags(tags, versionConstraint)
		fullyQualifiedImage := c.getFullyQualifiedImageRef(image)
		if c.DownloadNeeded {
			pulledTags = c.pullImages(fullyQualifiedImage, pulledTags)
		}

		if len(pulledTags) > 0 {
			browsers[browserName] = c.createVersions(browserName, fullyQualifiedImage, pulledTags)
		}
	}
	if c.DownloadNeeded {
		c.pullVideoRecorderImage()
	}
	return browsers
}

func parseRequestedBrowsers(logger *Logger, requestedBrowsers string) map[string][]*semver.Constraints {
	ret := make(map[string][]*semver.Constraints)
	if requestedBrowsers != "" {
		for _, section := range strings.Split(requestedBrowsers, semicolon) {
			pieces := strings.Split(section, colon)
			if len(pieces) >= 1 {
				browserName := strings.TrimSpace(pieces[0])
				if _, ok := ret[browserName]; !ok {
					ret[browserName] = []*semver.Constraints{}
				}
				if len(pieces) == 2 {
					versionConstraintString := strings.TrimSpace(pieces[1])

					versionConstraint, err := semver.NewConstraint(versionConstraintString)
					if err != nil {
						logger.Errorf(`Invalid version constraint %s: %v - ignoring browser "%s"...`, versionConstraintString, err, browserName)
						continue
					}
					ret[browserName] = append(ret[browserName], versionConstraint)
				}
			}
		}
	}
	return ret
}

func (c *DockerConfigurator) getBrowsersToIterate(requestedBrowsers map[string][]*semver.Constraints) map[string]string {
	defaultBrowsers := map[string]string{
		"firefox": "selenoid/firefox",
		"chrome":  "selenoid/chrome",
		"opera":   "selenoid/opera",
	}
	if len(requestedBrowsers) > 0 {
		if _, ok := requestedBrowsers[android]; ok {
			defaultBrowsers[android] = "selenoid/android"
		}
		if _, ok := requestedBrowsers[edge]; ok {
			defaultBrowsers[edge] = "browsers/edge"
		}
		ret := make(map[string]string)
		for browserName := range requestedBrowsers {
			if image, ok := defaultBrowsers[browserName]; ok {
				ret[browserName] = image
				continue
			}
			c.Errorf("Unsupported browser: %s", browserName)
		}

		return ret
	}
	return defaultBrowsers
}

func (c *DockerConfigurator) fetchImageTags(image string) []string {
	c.Pointf(`Fetching tags for image %v`, color.BlueString(image))
	reg := c.getRegistryClient()
	if reg == nil {
		c.Errorf(`Docker registry client not initialized`)
		return nil
	}
	tags, err := reg.Tags(image)
	if err != nil {
		c.Errorf(`Failed to fetch tags for image "%s": %v`, image, err)
		return nil
	}
	tagsWithoutLatest := filterOutLatest(tags)
	strSlice := Natural(tagsWithoutLatest)
	sort.Sort(sort.Reverse(strSlice))
	return tagsWithoutLatest
}

func filterOutLatest(tags []string) []string {
	var ret []string
	for _, tag := range tags {
		if !strings.HasPrefix(tag, Latest) {
			ret = append(ret, tag)
		}
	}
	return ret
}

func (c *DockerConfigurator) filterTags(tags []string, versionConstraints []*semver.Constraints) []string {
	if len(versionConstraints) > 0 {
		var ret []string
		for _, tag := range tags {
			version, err := semver.NewVersion(tag)
			if err != nil {
				c.Errorf("Skipping tag %s as it does not follow semantic versioning: %v", tag, err)
				continue
			}
			for _, vc := range versionConstraints {
				if vc.Check(version) {
					ret = append(ret, tag)
				}
			}
		}
		return ret
	} else if c.LastVersions > 0 && c.LastVersions <= len(tags) {
		return tags[:c.LastVersions]
	}
	return tags
}

func (c *DockerConfigurator) createVersions(browserName string, image string, tags []string) config.Versions {
	versions := config.Versions{
		Default:  tags[0],
		Versions: make(map[string]*config.Browser),
	}
	for _, tag := range tags {
		version := tag
		browser := &config.Browser{
			Image: imageWithTag(image, tag),
			Port:  "4444",
			Path:  "/",
		}
		if browserName == firefox || browserName == android || (browserName == opera && version == tag_1216) {
			browser.Path = "/wd/hub"
		}
		if c.Tmpfs > 0 {
			tmpfs := make(map[string]string)
			tmpfs["/tmp"] = fmt.Sprintf("size=%dm", c.Tmpfs)
			browser.Tmpfs = tmpfs
		}
		if c.ShmSize > 0 {
			browser.ShmSize, _ = units.RAMInBytes(fmt.Sprintf("%dm", c.ShmSize))
		}
		browserEnv := strings.Fields(c.BrowserEnv)
		if len(browserEnv) > 0 {
			browser.Env = browserEnv
		}
		versions.Versions[version] = browser
	}
	return versions
}

func imageWithTag(image string, tag string) string {
	return fmt.Sprintf("%s:%s", image, tag)
}

func (c *DockerConfigurator) pullImages(image string, tags []string) []string {
	var pulledTags []string
	ctx := context.Background()
	for _, tag := range tags {
		ref := imageWithTag(image, tag)
		if !c.pullImage(ctx, ref) {
			continue
		}
		pulledTags = append(pulledTags, tag)
	}
	return pulledTags
}

func (c *DockerConfigurator) pullVideoRecorderImage() {
	c.Titlef("Pulling video recorder image...")
	c.pullImage(context.Background(), c.getFullyQualifiedImageRef(videoRecorderImage))
}

func (c *DockerConfigurator) getFullyQualifiedImageRef(ref string) string {
	if c.registryHost != "" {
		return fmt.Sprintf("%s/%s", c.registryHost, ref)
	}
	return ref
}

// JSONMessage defines a message struct from docker.
type JSONMessage struct {
	Status          string        `json:"status,omitempty"`
	Progress        *JSONProgress `json:"progressDetail,omitempty"`
	ID              string        `json:"id,omitempty"`
	ProgressMessage string        `json:"progress,omitempty"` //deprecated
}

// JSONProgress describes a Progress. terminalFd is the fd of the current terminal,
// Start is the initial value for the operation. Current is the current status and
// value of the progress made towards Total. Total is the end value describing when
// we made 100% progress for an operation.
type JSONProgress struct {
	terminalFd uintptr
	Current    int64 `json:"current,omitempty"`
	Total      int64 `json:"total,omitempty"`
	Start      int64 `json:"start,omitempty"`
	// If true, don't show xB/yB
	HideCounts bool   `json:"hidecounts,omitempty"`
	Units      string `json:"units,omitempty"`
}

func (c *DockerConfigurator) pullImage(ctx context.Context, ref string) bool {
	c.Pointf("Pulling image %v", color.BlueString(ref))
	pullOptions := types.ImagePullOptions{}
	if c.authConfig != nil {
		buf, err := json.Marshal(c.authConfig)
		if err != nil {
			c.Errorf("Failed to prepare registry authentication config: %v", err)
		} else {
			pullOptions.RegistryAuth = base64.URLEncoding.EncodeToString(buf)
		}
	}
	resp, err := c.docker.ImagePull(ctx, ref, pullOptions)
	if err != nil {
		c.Errorf(`Failed to pull image "%s": %v`, ref, err)
		return false
	}
	defer resp.Close()

	var row JSONMessage

	scanner := bufio.NewScanner(resp)
	writer := rewriter.New(colorable.NewColorableStdout())

	for _ = ""; scanner.Scan(); {
		err := json.Unmarshal(scanner.Bytes(), &row)
		if err != nil {
			return false
		}

		select {
		case <-ctx.Done():
			{
				c.Errorf(`Pulling "%s" interrupted: %v`, ref, ctx.Err())
				return false
			}
		default:
			{
				if row.Progress != nil {
					if row.Progress.Current != row.Progress.Total {
						fmt.Fprintf(writer, "\t[%s]: %s %s\n", row.ID, row.Status, row.ProgressMessage)
					} else {
						fmt.Fprint(writer, "\r")
					}
				}

				writer.Flush()
			}
		}
	}

	if err := scanner.Err(); err != nil {
		c.Errorf(`Failed to pull image "%s": %v`, ref, color.RedString("%v", err))
	}
	return true
}

func (c *DockerConfigurator) IsRunning() bool {
	return c.getSelenoidContainer() != nil
}

func (c *DockerConfigurator) getSelenoidContainer() *types.Container {
	return c.getContainer(selenoidContainerName)
}

func (c *DockerConfigurator) IsUIRunning() bool {
	return c.getSelenoidUIContainer() != nil
}

func (c *DockerConfigurator) getSelenoidUIContainer() *types.Container {
	return c.getContainer(selenoidUIContainerName)
}

func (c *DockerConfigurator) getContainer(name string) *types.Container {
	f := filters.NewArgs()
	f.Add("name", fmt.Sprintf("^/%s$", name))
	containers, err := c.docker.ContainerList(context.Background(), types.ContainerListOptions{Filters: f})
	if err != nil {
		return nil
	}
	if len(containers) > 0 {
		return &containers[0]
	}
	return nil
}

func (c *DockerConfigurator) PrintArgs() error {
	image := c.getSelenoidImage()
	if image == nil {
		return errors.New("Selenoid image is not downloaded: this is probably a bug")
	}
	cfg := &containerConfig{
		Image:     image,
		Cmd:       []string{"--help"},
		PrintLogs: true,
	}
	return c.startContainer(cfg)
}

const (
	videoDirName = "video"
	logsDirName  = "logs"
	networkName  = "selenoid"
)

func (c *DockerConfigurator) Start() error {
	image := c.getSelenoidImage()
	if image == nil {
		return errors.New("Selenoid image is not downloaded: this is probably a bug")
	}

	volumeConfigDir := getVolumeConfigDir(c.ConfigDir, selenoidConfigDirElem)
	videoConfigDir := getVolumeConfigDir(filepath.Join(c.ConfigDir, videoDirName), append(selenoidConfigDirElem, videoDirName))
	logsConfigDir := getVolumeConfigDir(filepath.Join(c.ConfigDir, logsDirName), append(selenoidConfigDirElem, logsDirName))
	volumes := []string{
		fmt.Sprintf("%s:/etc/selenoid:ro,Z", volumeConfigDir),
		fmt.Sprintf("%s:/opt/selenoid/video:Z", videoConfigDir),
		fmt.Sprintf("%s:/opt/selenoid/logs:Z", logsConfigDir),
	}
	const dockerSocket = "/var/run/docker.sock"
	if isWindows() {
		//With two slashes. See https://stackoverflow.com/questions/36765138/bind-to-docker-socket-on-windows
		volumes = append(volumes, fmt.Sprintf("/%s:%s", dockerSocket, dockerSocket))
	} else if fileExists(dockerSocket) {
		volumes = append(volumes, fmt.Sprintf("%s:%s:Z", dockerSocket, dockerSocket))
	}

	cmd := []string{}
	overrideCmd := strings.Fields(c.Args)
	if len(overrideCmd) > 0 {
		cmd = overrideCmd
	}
	if !contains(cmd, "-conf") {
		cmd = append(cmd, "-conf", "/etc/selenoid/browsers.json")
	}
	if !contains(cmd, "-video-output-dir") && isVideoRecordingSupported(c.Logger, c.Version) {
		cmd = append(cmd, "-video-output-dir", "/opt/selenoid/video/")
	}
	if !contains(cmd, "-video-recorder-image") && isVideoRecordingSupported(c.Logger, c.Version) {
		cmd = append(cmd, "-video-recorder-image", c.getFullyQualifiedImageRef(videoRecorderImage))
	}
	if !c.DisableLogs && !contains(cmd, "-log-output-dir") && isLogSavingSupported(c.Logger, c.Version) {
		cmd = append(cmd, "-log-output-dir", "/opt/selenoid/logs/")
	}
	if !contains(cmd, "-container-network") {
		cmd = append(cmd, "-container-network", networkName)
	}

	overrideEnv := strings.Fields(c.Env)
	if !strings.Contains(c.Env, "OVERRIDE_VIDEO_OUTPUT_DIR") {
		overrideEnv = append(overrideEnv, fmt.Sprintf("OVERRIDE_VIDEO_OUTPUT_DIR=%s", videoConfigDir))
	}
	cfg := &containerConfig{
		Name:        selenoidContainerName,
		Image:       image,
		HostPort:    c.Port,
		ServicePort: SelenoidDefaultPort,
		Volumes:     volumes,
		Network:     networkName,
		Cmd:         cmd,
		OverrideEnv: overrideEnv,
		UserNS:      c.UserNS,
	}
	return c.startContainer(cfg)
}

func isVideoRecordingSupported(logger Logger, version string) bool {
	return isVersion(version, ">= 1.4.0", func(version string) {
		logger.Pointf(`Not enabling video feature because specified version "%s" is not semantic`, version)
	})
}

func isVersion(version string, condition string, notSemanticVersionCallback func(string)) bool {
	if version == Latest {
		return true
	}
	constraint, _ := semver.NewConstraint(condition)
	v, err := semver.NewVersion(version)
	if err != nil {
		notSemanticVersionCallback(version)
		return false
	}
	return constraint.Check(v)
}

func isLogSavingSupported(logger Logger, version string) bool {
	return isVersion(version, ">= 1.7.0", func(version string) {
		logger.Pointf(`Not enabling log saving feature because specified version "%s" is not semantic`, version)
	})
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func getVolumeConfigDir(defaultConfigDir string, elem []string) string {
	configDir := chooseVolumeConfigDir(defaultConfigDir, elem)
	if isWindows() { //A bit ugly, but conditional compilation is even worse
		return postProcessPath(configDir)
	}
	return configDir
}

// According to https://stackoverflow.com/questions/34161352/docker-sharing-a-volume-on-windows-with-docker-toolbox
func postProcessPath(path string) string {
	if len(path) >= 2 {
		replacedSlashes := strings.Replace(path, string("\\"), "/", -1)
		re := regexp.MustCompile("([A-Z]):(.+)")
		lowerCaseDriveLetter := strings.ToLower(re.ReplaceAllString(replacedSlashes, "$1"))
		pathTail := re.ReplaceAllString(replacedSlashes, "$2")
		return "/" + lowerCaseDriveLetter + pathTail
	}
	return path
}

func chooseVolumeConfigDir(defaultConfigDir string, elem []string) string {
	overrideHome := os.Getenv(overrideHome)
	if overrideHome != "" {
		return joinPaths(overrideHome, elem)
	}
	return defaultConfigDir
}

func (c *DockerConfigurator) PrintUIArgs() error {
	image := c.getSelenoidUIImage()
	if image == nil {
		return errors.New("Selenoid UI image is not downloaded: this is probably a bug")
	}
	cfg := &containerConfig{
		Image:     image,
		Cmd:       []string{"--help"},
		PrintLogs: true,
	}
	return c.startContainer(cfg)
}

func (c *DockerConfigurator) StartUI() error {
	image := c.getSelenoidUIImage()
	if image == nil {
		return errors.New("Selenoid UI image is not downloaded: this is probably a bug")
	}

	var cmd, candidates []string
	var selenoidUri string
containers:
	for _, containerName := range []string{
		selenoidContainerName, ggrUIContainerName,
	} {
		if ctr := c.getContainer(containerName); ctr != nil {
			for _, p := range ctr.Ports {
				if p.PublicPort != 0 {
					selenoidUri = fmt.Sprintf("--selenoid-uri=http://%s:%d", containerName, p.PublicPort)
					candidates = []string{containerName}
					break containers
				}
			}
		}
	}
	overrideCmd := strings.Fields(c.Args)
	if len(overrideCmd) > 0 {
		cmd = overrideCmd
	}
	if !contains(cmd, "--selenoid-uri") {
		cmd = append(cmd, selenoidUri)
	}

	if len(candidates) == 0 {
		c.Errorf("Neither Selenoid nor Ggr UI is started. Selenoid UI may not work.")
	}

	overrideEnv := strings.Fields(c.Env)
	cfg := &containerConfig{
		Name:        selenoidUIContainerName,
		Image:       image,
		HostPort:    c.Port,
		ServicePort: SelenoidUIDefaultPort,
		Network:     networkName,
		Cmd:         cmd,
		OverrideEnv: overrideEnv,
		UserNS:      c.UserNS,
	}
	return c.startContainer(cfg)
}

func validateEnviron(envs []string) []string {
	validEnv := []string{}
	for _, e := range envs {
		k := strings.Split(e, "=")
		if len(k[0]) != 0 {
			validEnv = append(validEnv, e)
		}
	}
	return validEnv
}

type containerConfig struct {
	Name        string
	Image       *types.ImageSummary
	HostPort    int
	ServicePort int
	Volumes     []string
	Network     string
	Cmd         []string
	OverrideEnv []string
	UserNS      string
	PrintLogs   bool
}

func (c *DockerConfigurator) startContainer(cfg *containerConfig) error {
	ctx := context.Background()
	env := validateEnviron(os.Environ())
	env = append(env, fmt.Sprintf("TZ=%s", time.Local))
	if len(cfg.OverrideEnv) > 0 {
		env = cfg.OverrideEnv
	}
	if !contains(env, dockerApiVersion) {
		env = append(env, fmt.Sprintf("%s=%s", dockerApiVersion, c.docker.ClientVersion()))
	}
	servicePortString := strconv.Itoa(cfg.ServicePort)
	port, err := nat.NewPort("tcp", servicePortString)
	if err != nil {
		return fmt.Errorf("failed to init port: %v", err)
	}

	err = c.createNetworkIfNeeded(cfg.Network)
	if err != nil {
		return fmt.Errorf("failed to configure container network: %v", err)
	}
	containerConfig := container.Config{
		Hostname: "localhost",
		Image:    cfg.Image.RepoTags[0],
		Env:      env,
	}
	if cfg.ServicePort > 0 {
		containerConfig.ExposedPorts = map[nat.Port]struct{}{port: {}}
	}
	if len(cfg.Cmd) > 0 {
		containerConfig.Cmd = strslice.StrSlice(cfg.Cmd)
	}
	hostConfig := container.HostConfig{
		Binds:       cfg.Volumes,
		NetworkMode: networkName,
	}
	if cfg.UserNS != "" {
		mode := container.UsernsMode(cfg.UserNS)
		if !mode.Valid() {
			return fmt.Errorf("invalid userns value: %s", cfg.UserNS)
		}
		hostConfig.UsernsMode = mode
	}
	if cfg.PrintLogs {
		containerConfig.Tty = true
	} else {
		hostConfig.RestartPolicy = container.RestartPolicy{
			Name: "always",
		}
	}
	if cfg.HostPort > 0 && cfg.ServicePort > 0 {
		hostPortString := strconv.Itoa(cfg.HostPort)
		portBindings := nat.PortMap{}
		portBindings[port] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostPortString}}
		hostConfig.PortBindings = portBindings
	}
	ctr, err := c.docker.ContainerCreate(ctx,
		&containerConfig,
		&hostConfig,
		&network.NetworkingConfig{}, nil, cfg.Name)
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}
	err = c.docker.ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{})
	if err != nil {
		c.removeContainer(ctr.ID)
		return fmt.Errorf("failed to start container: %v", err)
	}
	if cfg.PrintLogs {
		defer c.removeContainer(ctr.ID)
		r, err := c.docker.ContainerLogs(ctx, ctr.ID, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
		})
		if err != nil {
			return fmt.Errorf("failed to read container logs: %v", err)
		}
		defer r.Close()
		io.Copy(os.Stderr, r)
	}
	return nil
}

func (c *DockerConfigurator) createNetworkIfNeeded(networkName string) error {
	ctx := context.Background()
	_, err := c.docker.NetworkInspect(ctx, networkName, types.NetworkInspectOptions{})
	if err != nil {
		_, err = c.docker.NetworkCreate(ctx, networkName, types.NetworkCreate{CheckDuplicate: true})
		if err != nil {
			return fmt.Errorf("failed to create custom network %s: %v", networkName, err)
		}
	}
	return nil
}

func (c *DockerConfigurator) removeContainer(id string) error {
	ctx := context.Background()
	if c.Graceful {
		err := c.docker.ContainerStop(ctx, id, &c.GracefulTimeout)
		if err == nil {
			return c.docker.ContainerRemove(ctx, id, types.ContainerRemoveOptions{RemoveVolumes: true})
		}
		return err
	}
	return c.docker.ContainerRemove(ctx, id, types.ContainerRemoveOptions{RemoveVolumes: true, Force: true})
}

func (c *DockerConfigurator) Stop() error {
	sc := c.getSelenoidContainer()
	if sc != nil {
		err := c.removeContainer(sc.ID)
		if err != nil {
			return fmt.Errorf("failed to stop Selenoid container: %v", err)
		}
	}
	return nil
}

func (c *DockerConfigurator) StopUI() error {
	uc := c.getSelenoidUIContainer()
	if uc != nil {
		err := c.removeContainer(uc.ID)
		if err != nil {
			return fmt.Errorf("failed to stop Selenoid UI container: %v", err)
		}
	}
	return nil
}
