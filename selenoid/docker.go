package selenoid

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"sort"

	"errors"
	"github.com/aerokube/selenoid/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/heroku/docker-registry-client/registry"
	"os"
	"strconv"
	"strings"
	"time"

	"regexp"
	"runtime"
	. "vbom.ml/util/sortorder"
)

const (
	Latest                  = "latest"
	firefox                 = "firefox"
	opera                   = "opera"
	tag_1216                = "12.16"
	selenoidImage           = "aerokube/selenoid"
	selenoidUIImage         = "aerokube/selenoid-ui"
	selenoidContainerName   = "selenoid"
	selenoidContainerPort   = 4444
	selenoidUIContainerName = "selenoid-ui"
	selenoidUIContainerPort = 8080
	overrideHome            = "OVERRIDE_HOME"
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
	LastVersions int
	Pull         bool
	RegistryUrl  string
	Tmpfs        int
	VNC          bool
	docker       *client.Client
	reg          *registry.Registry
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
		RegistryUrl:            config.RegistryUrl,
		LastVersions:           config.LastVersions,
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
	err = c.initRegistryClient()
	if err != nil {
		return nil, fmt.Errorf("new configurator: %v", err)
	}
	return c, nil
}

func (c *DockerConfigurator) initDockerClient() error {
	docker, err := client.NewEnvClient()
	if err != nil {
		return fmt.Errorf("failed to init Docker client: %v", err)
	}
	c.docker = docker
	return nil
}

func (c *DockerConfigurator) initRegistryClient() error {
	reg, err := registry.New(c.RegistryUrl, "", "")
	if err != nil {
		return fmt.Errorf("Docker Registry is not available: %v", err)
	}
	c.reg = reg
	return nil
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
		c.Printf("Using Selenoid image: %s (%s)", selenoidImage.RepoTags[0], selenoidImage.ID)
	} else {
		c.Printf("Selenoid image is not present")
	}
	configPath := getSelenoidConfigPath(c.ConfigDir)
	c.Printf("Selenoid configuration directory is %s", c.ConfigDir)
	if fileExists(configPath) {
		c.Printf("Selenoid configuration file is %s", configPath)
	} else {
		c.Printf("Selenoid is not configured")
	}
	selenoidContainer := c.getSelenoidContainer()
	if selenoidContainer != nil {
		c.Printf("Selenoid container is running: %s (%s)", selenoidContainerName, selenoidContainer.ID)
	} else {
		c.Printf("Selenoid container is not running")
	}
}

func (c *DockerConfigurator) UIStatus() {
	selenoidUIImage := c.getSelenoidUIImage()
	if selenoidUIImage != nil {
		c.Printf("Using Selenoid UI image: %s (%s)", selenoidUIImage.RepoTags[0], selenoidUIImage.ID)
	} else {
		c.Printf("Selenoid UI image is not present")
	}
	selenoidUIContainer := c.getSelenoidUIContainer()
	if selenoidUIContainer != nil {
		c.Printf("Selenoid UI container is running: %s (%s)", selenoidUIContainerName, selenoidUIContainer.ID)
	} else {
		c.Printf("Selenoid UI container is not running")
	}
}

func (c *DockerConfigurator) IsDownloaded() bool {
	return c.getSelenoidImage() != nil
}

func (c *DockerConfigurator) getSelenoidImage() *types.ImageSummary {
	return c.getImage(selenoidImage)
}

func (c *DockerConfigurator) IsUIDownloaded() bool {
	return c.getSelenoidUIImage() != nil
}

func (c *DockerConfigurator) getSelenoidUIImage() *types.ImageSummary {
	return c.getImage(selenoidUIImage)
}

func (c *DockerConfigurator) getImage(name string) *types.ImageSummary {
	images, err := c.docker.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		c.Printf("Failed to list images: %v\n", err)
		return nil
	}
	for _, img := range images {
		const colon = ":"
		for _, tag := range img.RepoTags {
			imageName := strings.Split(tag, colon)[0]
			if strings.HasSuffix(imageName, name) {
				return &img
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
	ref := imageName
	if version != Latest {
		ref = fmt.Sprintf("%s:%s", ref, version)
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
	cfg := c.createConfig()
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %v\n", err)
	}
	err = c.createConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v\n", err)
	}
	return &cfg, ioutil.WriteFile(getSelenoidConfigPath(c.ConfigDir), data, 0644)
}

func (c *DockerConfigurator) createConfig() SelenoidConfig {
	supportedBrowsers := c.getSupportedBrowsers()
	browsers := make(map[string]config.Versions)
	browsersToIterate := supportedBrowsers
	if c.Browsers != "" {
		requestedBrowsers := strings.Split(c.Browsers, comma)
		if len(requestedBrowsers) > 0 {
			browsersToIterate = make(map[string]string)
			for _, rb := range requestedBrowsers {
				if image, ok := supportedBrowsers[rb]; ok {
					browsersToIterate[rb] = image
					continue
				}
				c.Printf("Unsupported browser: %s\n", rb)
			}
		}
	}
	for browserName, image := range browsersToIterate {
		c.Printf("Processing browser \"%s\"...\n", browserName)
		tags := c.fetchImageTags(image)
		image, tags = c.preProcessImageTags(image, browserName, tags)
		pulledTags := tags
		if c.DownloadNeeded {
			pulledTags = c.pullImages(image, tags)
		} else if c.LastVersions > 0 && c.LastVersions <= len(tags) {
			pulledTags = tags[:c.LastVersions]
		}

		if len(pulledTags) > 0 {
			browsers[browserName] = c.createVersions(browserName, image, pulledTags)
		}
	}
	return browsers
}

func (c *DockerConfigurator) getSupportedBrowsers() map[string]string {
	return map[string]string{
		"firefox": "selenoid/firefox",
		"chrome":  "selenoid/chrome",
		"opera":   "selenoid/opera",
	}
}

func (c *DockerConfigurator) fetchImageTags(image string) []string {
	c.Printf("Fetching tags for image \"%s\"...\n", image)
	tags, err := c.reg.Tags(image)
	if err != nil {
		c.Printf("Failed to fetch tags for image \"%s\"\n", image)
		return nil
	}
	tagsWithoutLatest := filterOutLatest(tags)
	strSlice := Natural(tagsWithoutLatest)
	sort.Sort(sort.Reverse(strSlice))
	return tagsWithoutLatest
}

func filterOutLatest(tags []string) []string {
	ret := []string{}
	for _, tag := range tags {
		if !strings.HasPrefix(tag, Latest) {
			ret = append(ret, tag)
		}
	}
	return ret
}

func (c *DockerConfigurator) createVersions(browserName string, image string, tags []string) config.Versions {
	versions := config.Versions{
		Default:  c.getVersionFromTag(browserName, tags[0]),
		Versions: make(map[string]*config.Browser),
	}
	for _, tag := range tags {
		version := c.getVersionFromTag(browserName, tag)
		browser := &config.Browser{
			Image: imageWithTag(image, tag),
			Port:  "4444",
			Path:  "/",
		}
		if browserName == firefox || (browserName == opera && version == tag_1216) {
			browser.Path = "/wd/hub"
		}
		if c.Tmpfs > 0 {
			tmpfs := make(map[string]string)
			tmpfs["/tmp"] = fmt.Sprintf("size=%dm", c.Tmpfs)
			browser.Tmpfs = tmpfs
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
	pulledTags := []string{}
	ctx := context.Background()
loop:
	for _, tag := range tags {
		ref := imageWithTag(image, tag)
		c.Printf("Pulling image \"%s\"...\n", ref)
		if !c.pullImage(ctx, ref) {
			continue
		}
		pulledTags = append(pulledTags, tag)
		if c.LastVersions > 0 && len(pulledTags) == c.LastVersions {
			break loop
		}
	}
	return pulledTags
}

func (c *DockerConfigurator) preProcessImageTags(image string, browserName string, tags []string) (string, []string) {
	imageToProcess := image
	tagsToProcess := tags
	if c.VNC {
		c.Printf("Requested to download VNC images...\n")
		imageToProcess = "selenoid/vnc"
		tagsToProcess = []string{}
		for _, tag := range tags {
			tagsToProcess = append(tagsToProcess, createVNCTag(browserName, tag))
		}
	}
	return imageToProcess, tagsToProcess
}

func createVNCTag(browserName string, version string) string {
	return fmt.Sprintf("%s_%s", browserName, version)
}

func (c *DockerConfigurator) getVersionFromTag(browserName string, tag string) string {
	if c.VNC {
		return strings.TrimPrefix(tag, browserName+"_")
	}
	return tag
}

func (c *DockerConfigurator) pullImage(ctx context.Context, ref string) bool {
	resp, err := c.docker.ImagePull(ctx, ref, types.ImagePullOptions{})
	if err != nil {
		c.Printf("Failed to pull image \"%s\": %v", ref, err)
		return false
	}
	defer resp.Close()
	var row struct {
		Id     string `json:"id"`
		Status string `json:"status"`
	}
	scanner := bufio.NewScanner(resp)
	for prev := ""; scanner.Scan(); {
		err := json.Unmarshal(scanner.Bytes(), &row)
		if err != nil {
			return false
		}
		select {
		case <-ctx.Done():
			{
				c.Printf("Pulling \"%s\" interrupted: %v", ref, ctx.Err())
				return false
			}
		default:
			{
				if prev != row.Status {
					prev = row.Status
					c.Printf("%s: %s\n", row.Status, row.Id)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		c.Printf("Failed to pull image \"%s\": %v", ref, err)
	}
	return true
}

func (c *DockerConfigurator) IsRunning() bool {
	return c.getSelenoidContainer() != nil
}

func (c *DockerConfigurator) getSelenoidContainer() *types.Container {
	return c.getContainer(selenoidContainerName, selenoidContainerPort)
}

func (c *DockerConfigurator) IsUIRunning() bool {
	return c.getSelenoidUIContainer() != nil
}

func (c *DockerConfigurator) getSelenoidUIContainer() *types.Container {
	return c.getContainer(selenoidUIContainerName, selenoidUIContainerPort)
}

func (c *DockerConfigurator) getContainer(name string, port uint16) *types.Container {
	f := filters.NewArgs()
	f.Add("name", name)
	containers, err := c.docker.ContainerList(context.Background(), types.ContainerListOptions{Filters: f})
	if err != nil {
		return nil
	}
	for _, c := range containers {
		for _, p := range c.Ports {
			if p.PublicPort == port {
				return &c
			}
		}
	}
	return nil
}

func (c *DockerConfigurator) Start() error {
	image := c.getSelenoidImage()
	if image == nil {
		return errors.New("Selenoid image is not downloaded: this is probably a bug")
	}

	volumeConfigDir := c.getVolumeConfigDir(selenoidConfigDirElem)
	volumes := []string{fmt.Sprintf("%s:/etc/selenoid:ro", volumeConfigDir)}
	const dockerSocket = "/var/run/docker.sock"
	if fileExists(dockerSocket) {
		volumes = append(volumes, fmt.Sprintf("%s:%s", dockerSocket, dockerSocket))
	}

	overrideEnv := strings.Fields(c.Env)
	return c.startContainer(selenoidContainerName, image, selenoidContainerPort, volumes, []string{}, strings.Fields(c.Args), overrideEnv)
}

func (c *DockerConfigurator) getVolumeConfigDir(elem []string) string {
	configDir := chooseVolumeConfigDir(c.ConfigDir, elem)
	if runtime.GOOS == "windows" { //A bit ugly, but conditional compilation is even worse
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

func (c *DockerConfigurator) StartUI() error {
	image := c.getSelenoidUIImage()
	if image == nil {
		return errors.New("Selenoid UI image is not downloaded: this is probably a bug")
	}

	links := []string{selenoidContainerName}

	cmd := []string{}
	overrideCmd := strings.Fields(c.Args)
	if len(overrideCmd) > 0 {
		cmd = overrideCmd
	}
	if !contains(cmd, "--selenoid-uri") {
		cmd = append(cmd, fmt.Sprintf("--selenoid-uri=http://%s:%d", selenoidContainerName, selenoidContainerPort))
	}

	overrideEnv := strings.Fields(c.Env)
	return c.startContainer(selenoidUIContainerName, image, selenoidUIContainerPort, []string{}, links, cmd, overrideEnv)
}

func ValidEnviron() []string {
	env := os.Environ()
	validEnv := make([]string, 0, 50)
	for _, e := range env {
		k := strings.Split(e, "=")
		if len(k[0]) != 0 {
			validEnv = append(validEnv, e)
		}
	}
	return validEnv
}

func (c *DockerConfigurator) startContainer(name string, image *types.ImageSummary, forwardedPort int, volumes []string, links []string, cmd []string, envOverride []string) error {
	env := ValidEnviron()
	env = append(env, fmt.Sprintf("TZ=%s", time.Local))
	if len(envOverride) > 0 {
		env = envOverride
	}
	portString := strconv.Itoa(forwardedPort)
	port, err := nat.NewPort("tcp", portString)
	if err != nil {
		return fmt.Errorf("failed to init port: %v", err)
	}
	exposedPorts := map[nat.Port]struct{}{port: {}}
	portBindings := nat.PortMap{}
	portBindings[port] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: portString}}
	ctx := context.Background()
	containerConfig := container.Config{
		Hostname:     "localhost",
		Image:        image.RepoTags[0],
		Env:          env,
		ExposedPorts: exposedPorts,
	}
	if len(cmd) > 0 {
		containerConfig.Cmd = strslice.StrSlice(cmd)
	}
	ctr, err := c.docker.ContainerCreate(ctx,
		&containerConfig,
		&container.HostConfig{
			Binds:        volumes,
			Links:        links,
			PortBindings: portBindings,
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
		},
		&network.NetworkingConfig{}, name)
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}
	err = c.docker.ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{})
	if err != nil {
		c.removeContainer(ctr.ID)
		return fmt.Errorf("failed to start container: %v", err)
	}
	return nil
}

func (c *DockerConfigurator) removeContainer(id string) error {
	return c.docker.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{RemoveVolumes: true, Force: true})
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
