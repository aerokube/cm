package selenoid

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aerokube/selenoid/config"
	"github.com/google/go-github/github"
	"github.com/mitchellh/go-ps"
	"gopkg.in/cheggaaa/pb.v1"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	zipMagicHeader  = "504b"
	gzipMagicHeader = "1f8b"
	comma           = ","
	owner           = "aerokube"
	selenoidRepo    = "selenoid"
	selenoidUIRepo  = "selenoid-ui"
)

type Browsers map[string]Browser

type Browser struct {
	Command string `json:"command"`
	Files   Files  `json:"files"`
}

type Files map[string]Architectures

type Architectures map[string]Driver

type Driver struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

type downloadedDriver struct {
	BrowserName string
	Command     string
}

type DriversConfigurator struct {
	Logger
	ConfigDirAware
	VersionAware
	DownloadAware
	ArgsAware
	RequestedBrowsersAware
	Browsers        string
	BrowsersJsonUrl string

	GithubBaseUrl string
	OS            string
	Arch          string
}

func NewDriversConfigurator(config *LifecycleConfig) *DriversConfigurator {
	return &DriversConfigurator{
		Logger:                 Logger{Quiet: config.Quiet},
		ConfigDirAware:         ConfigDirAware{ConfigDir: config.ConfigDir},
		VersionAware:           VersionAware{Version: config.Version},
		ArgsAware:              ArgsAware{Args: config.Args},
		DownloadAware:          DownloadAware{DownloadNeeded: config.Download},
		RequestedBrowsersAware: RequestedBrowsersAware{Browsers: config.Browsers},
		BrowsersJsonUrl:        config.BrowsersJsonUrl,
		Browsers:               config.Browsers,
		GithubBaseUrl:          config.GithubBaseUrl,
		OS:                     config.OS,
		Arch:                   config.Arch,
	}
}

func (d *DriversConfigurator) Status() {
	binaryPath := d.getSelenoidBinaryPath()
	if fileExists(binaryPath) {
		d.Printf("Selenoid binary is %s", binaryPath)
	} else {
		d.Printf("Selenoid binary is not downloaded")
	}
	configPath := getSelenoidConfigPath(d.ConfigDir)
	d.Printf("Selenoid configuration directory is %s", d.ConfigDir)
	if fileExists(configPath) {
		d.Printf("Selenoid configuration file is %s", configPath)
	} else {
		d.Printf("Selenoid is not configured")
	}
	selenoidProcesses := findSelenoidProcesses()
	if len(selenoidProcesses) > 0 {
		d.Printf("Selenoid is running as process %d", selenoidProcesses[0].Pid)
	} else {
		d.Printf("Selenoid is not running")
	}
}

func (d *DriversConfigurator) UIStatus() {
	binaryPath := d.getSelenoidUIBinaryPath()
	if fileExists(binaryPath) {
		d.Printf("Selenoid UI binary is %s", binaryPath)
	} else {
		d.Printf("Selenoid UI binary is not downloaded")
	}
	selenoidUIProcesses := findSelenoidUIProcesses()
	if len(selenoidUIProcesses) > 0 {
		d.Printf("Selenoid UI is running as process %d", selenoidUIProcesses[0].Pid)
	} else {
		d.Printf("Selenoid UI is not running")
	}
}

func (d *DriversConfigurator) IsDownloaded() bool {
	return fileExists(d.getSelenoidBinaryPath())
}

func (d *DriversConfigurator) getSelenoidBinaryPath() string {
	return d.getBinaryPath(getSelenoidReleaseFileName())
}

func (d *DriversConfigurator) IsUIDownloaded() bool {
	return fileExists(d.getSelenoidUIBinaryPath())
}

func (d *DriversConfigurator) getSelenoidUIBinaryPath() string {
	return d.getBinaryPath(getSelenoidUIReleaseFileName())
}

func (d *DriversConfigurator) getBinaryPath(fileName string) string {
	return filepath.Join(d.ConfigDir, fileName)
}

func getSelenoidConfigPath(outputDir string) string {
	return filepath.Join(outputDir, "browsers.json")
}

func (d *DriversConfigurator) Download() (string, error) {
	u, err := d.getSelenoidUrl()
	if err != nil {
		return "", fmt.Errorf("failed to get Selenoid download URL for arch = %s and version = %s: %v\n", d.Arch, d.Version, err)
	}
	err = d.createConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to create Selenoid config directory: %v\n", err)
	}
	if d.IsRunning() {
		d.Printf("Stopping Selenoid to overwrite its binary...")
		err := d.Stop()
		if err != nil {
			return "", fmt.Errorf("failed to stop Selenoid: %v\n", err)
		}
	}
	outputFile, err := d.downloadFile(u)
	if err != nil {
		return "", fmt.Errorf("failed to download Selenoid for arch = %s and version = %s: %v\n", d.Arch, d.Version, err)
	}
	d.Printf("Successfully downloaded Selenoid to %s\n", outputFile)
	return outputFile, nil
}
func (d *DriversConfigurator) getSelenoidUrl() (string, error) {
	d.Printf("Getting Selenoid release information for version: %s\n", d.Version)
	return d.getUrl(selenoidRepo, fmt.Errorf("Selenoid binary for %s %s is not available for specified release: %s\n", strings.Title(d.OS), d.Arch, d.Version))
}

func (d *DriversConfigurator) DownloadUI() (string, error) {
	u, err := d.getSelenoidUIUrl()
	if err != nil {
		return "", fmt.Errorf("failed to get download URL for arch = %s and version = %s: %v\n", d.Arch, d.Version, err)
	}
	err = d.createConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to create Selenoid UI config directory: %v\n", err)
	}
	if d.IsUIRunning() {
		d.Printf("Stopping Selenoid UI to overwrite its binary...")
		err := d.StopUI()
		if err != nil {
			return "", fmt.Errorf("failed to stop Selenoid UI: %v\n", err)
		}
	}
	outputFile, err := d.downloadFile(u)
	if err != nil {
		return "", fmt.Errorf("failed to download Selenoid UI for arch = %s and version = %s: %v\n", d.Arch, d.Version, err)
	}
	d.Printf("Successfully downloaded Selenoid UI to %s\n", outputFile)
	return outputFile, nil
}

func (d *DriversConfigurator) getSelenoidUIUrl() (string, error) {
	d.Printf("Getting Selenoid UI release information for version: %s\n", d.Version)
	return d.getUrl(selenoidUIRepo, fmt.Errorf("Selenoid UI binary for %s %s is not available for specified release: %s\n", strings.Title(d.OS), d.Arch, d.Version))
}

func (d *DriversConfigurator) getUrl(repo string, missingBinaryError error) (string, error) {
	ctx := context.Background()
	client := github.NewClient(nil)
	if d.GithubBaseUrl != "" {
		u, err := url.Parse(d.GithubBaseUrl)
		if err != nil {
			return "", fmt.Errorf("invalid Github base url [%s]: %v\n", d.GithubBaseUrl, err)
		}
		client.BaseURL = u
	}
	var release *github.RepositoryRelease
	var err error
	if d.Version != Latest {
		release, _, err = client.Repositories.GetReleaseByTag(ctx, owner, repo, d.Version)
	} else {
		release, _, err = client.Repositories.GetLatestRelease(ctx, owner, repo)
	}

	if err != nil {
		return "", err
	}

	if release == nil {
		return "", fmt.Errorf("unknown release: %s\n", d.Version)
	}

	for _, asset := range release.Assets {
		assetName := *(asset.Name)
		if strings.Contains(assetName, d.OS) && strings.Contains(assetName, d.Arch) {
			return *(asset.BrowserDownloadURL), nil
		}
	}
	return "", missingBinaryError
}

func (d *DriversConfigurator) downloadFile(url string) (string, error) {
	d.Printf("Downloading Selenoid release from %s\n", url)
	outputPath := d.getSelenoidBinaryPath()
	f, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return "", err
	}
	defer f.Close()

	err = downloadFileWithProgressBar(url, f)
	if err != nil {
		return "", err
	}
	d.Printf("Selenoid binary saved to %s\n", outputPath)
	return outputPath, nil
}

func (d *DriversConfigurator) IsConfigured() bool {
	return fileExists(getSelenoidConfigPath(d.ConfigDir))
}

func (d *DriversConfigurator) Configure() (*SelenoidConfig, error) {
	browsers, err := d.loadAvailableBrowsers()
	if err != nil {
		return nil, fmt.Errorf("failed to load available browsers: %v\n", err)
	}
	err = d.createConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v\n", err)
	}
	downloadedDrivers := d.downloadDrivers(browsers, d.ConfigDir)
	cfg := generateConfig(downloadedDrivers)
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return &cfg, fmt.Errorf("failed to marshal json: %v\n", err)
	}
	return &cfg, ioutil.WriteFile(getSelenoidConfigPath(d.ConfigDir), data, 0644)
}

func generateConfig(downloadedDrivers []downloadedDriver) SelenoidConfig {
	browsers := make(SelenoidConfig)
	for _, dd := range downloadedDrivers {
		cmd := strings.Fields(dd.Command)
		versions := config.Versions{
			Default: Latest,
			Versions: map[string]*config.Browser{
				Latest: {
					Image: cmd,
					Path:  "/",
				},
			},
		}
		browsers[dd.BrowserName] = versions
	}
	return browsers
}

func (d *DriversConfigurator) loadAvailableBrowsers() (*Browsers, error) {
	jsonUrl := d.BrowsersJsonUrl
	d.Printf("Downloading browser data from: %s\n", jsonUrl)
	data, err := downloadFile(jsonUrl)
	if err != nil {
		d.Printf("Browsers data download error: %v\n", err)
		return nil, err
	}
	var browsers Browsers
	err = json.Unmarshal(data, &browsers)
	if err != nil {
		d.Printf("Browsers data read error: %v\n", err)
		return nil, err
	}
	return &browsers, nil
}

func downloadFile(url string) ([]byte, error) {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	err := downloadFileWithProgressBar(url, w)
	if err != nil {
		return nil, err
	}
	w.Flush()
	return b.Bytes(), nil
}

func downloadFileWithProgressBar(url string, w io.Writer) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("file download error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected response code: %d", resp.StatusCode)
	}

	contentLength := int(resp.ContentLength)
	writer := w

	if contentLength > 0 {
		bar := pb.New(contentLength).SetUnits(pb.U_BYTES)
		bar.Output = os.Stderr
		bar.Start()
		defer bar.Finish()
		writer = io.MultiWriter(w, bar)
	}

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}
	return nil
}

func (d *DriversConfigurator) downloadDriver(driver *Driver, dir string) (string, error) {
	if d.DownloadNeeded {
		d.Printf("Downloading driver from %s...\n", driver.URL)
		data, err := downloadFile(driver.URL)
		if err != nil {
			return "", fmt.Errorf("failed to download driver archive: %v\n", err)
		}
		d.Printf("Unpacking archive to %s...\n", dir)
		return extractFile(data, driver.Filename, dir)
	}
	return filepath.Join(dir, driver.Filename), nil
}

func getMagicHeader(data []byte) string {
	if len(data) >= 2 {
		return hex.EncodeToString(data[:2])
	}
	return ""
}

func isZipFile(data []byte) bool {
	return getMagicHeader(data) == zipMagicHeader
}

func isTarGzFile(data []byte) bool {
	return getMagicHeader(data) == gzipMagicHeader
}

func extractFile(data []byte, filename string, outputDir string) (string, error) {
	if isZipFile(data) {
		return unzip(data, filename, outputDir)
	} else if isTarGzFile(data) {
		return untar(data, filename, outputDir)
	}
	return "", errors.New("Unknown archive type")
}

// Based on http://stackoverflow.com/questions/20357223/easy-way-to-unzip-file-with-golang
func unzip(data []byte, fileName string, outputDir string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) (string, error) {
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()

		outputPath := filepath.Join(outputDir, f.Name)

		if f.FileInfo().IsDir() {
			return "", fmt.Errorf("can only unzip files but %s is a directory", f.Name)
		}

		err = outputFile(outputPath, f.Mode(), rc)
		if err != nil {
			return "", err
		}
		return outputPath, nil
	}

	if err == nil {
		for _, f := range zr.File {
			if f.Name == fileName {
				return extractAndWriteFile(f)
			}
		}
		err = fmt.Errorf("file %s does not exist in archive", fileName)
	}

	return "", err
}

// Based on https://medium.com/@skdomino/taring-untaring-files-in-go-6b07cf56bc07
func untar(data []byte, fileName string, outputDir string) (string, error) {

	gzr, err := gzip.NewReader(bytes.NewReader(data))
	defer gzr.Close()

	extractAndWriteFile := func(tr *tar.Reader, header *tar.Header) (string, error) {

		outputPath := filepath.Join(outputDir, header.Name)

		if header.Typeflag == tar.TypeDir {
			return "", fmt.Errorf("can only untar files but %s is a directory", header.Name)
		}

		err = outputFile(outputPath, os.FileMode(header.Mode), tr)
		if err != nil {
			return "", err
		}
		return outputPath, nil
	}

	if err == nil {
		tr := tar.NewReader(gzr)

		for {
			header, err := tr.Next()
			switch {
			case err == io.EOF:
				break
			case err != nil:
				return "", err
			case header == nil:
				continue
			}
			return extractAndWriteFile(tr, header)
		}
		err = fmt.Errorf("file %s does not exist in archive", fileName)
	}

	return "", err
}

func outputFile(outputPath string, mode os.FileMode, r io.Reader) error {
	os.MkdirAll(filepath.Dir(outputPath), 0755)
	f, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}
	return nil
}

func (d *DriversConfigurator) downloadDrivers(browsers *Browsers, configDir string) []downloadedDriver {
	ret := []downloadedDriver{}
	browsersToIterate := *browsers
	if d.Browsers != "" {
		requestedBrowsers := strings.Split(d.Browsers, comma)
		if len(requestedBrowsers) > 0 {
			browsersToIterate = make(Browsers)
			for _, rb := range requestedBrowsers {
				if browser, ok := (*browsers)[rb]; ok {
					browsersToIterate[rb] = browser
					continue
				}
				d.Printf("Unsupported browser: %s\n", rb)
			}
		}
	}

loop:
	for browserName, browser := range browsersToIterate {
		goos := runtime.GOOS
		goarch := runtime.GOARCH
		if architectures, ok := browser.Files[goos]; ok {
			if driver, ok := architectures[goarch]; ok {
				d.Printf("Processing %s...\n", strings.Title(browserName))
				driverPath, err := d.downloadDriver(&driver, configDir)
				if err != nil {
					d.Printf("Failed to download %s driver: %v\n", strings.Title(browserName), err)
					continue loop
				}
				command := fmt.Sprintf(browser.Command, driverPath)
				ret = append(ret, downloadedDriver{
					BrowserName: browserName,
					Command:     command,
				})
			}
		}
	}
	return ret
}

func (d *DriversConfigurator) IsRunning() bool {
	selenoidProcesses := findSelenoidProcesses()
	return len(selenoidProcesses) > 0
}

func (d *DriversConfigurator) IsUIRunning() bool {
	selenoidUIProcesses := findSelenoidUIProcesses()
	return len(selenoidUIProcesses) > 0
}

func (d *DriversConfigurator) Start() error {
	args := []string{
		"-conf", getSelenoidConfigPath(d.ConfigDir),
		"-disable-docker",
	}
	overrideArgs := strings.Fields(d.Args)
	if len(overrideArgs) > 0 {
		args = overrideArgs
	}
	return runCommand(d.getSelenoidBinaryPath(), args)
}

func (d *DriversConfigurator) StartUI() error {
	args := strings.Fields(d.Args)
	return runCommand(d.getSelenoidUIBinaryPath(), args)
}

var killFunc func(os.Process) error = func(p os.Process) error {
	return p.Kill()
}

func (d *DriversConfigurator) Stop() error {
	return killAllProcesses(findSelenoidProcesses())
}

func (d *DriversConfigurator) StopUI() error {
	return killAllProcesses(findSelenoidUIProcesses())
}

func killAllProcesses(processes []os.Process) error {
	for _, p := range processes {
		err := killFunc(p)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DriversConfigurator) Close() error {
	//Does nothing
	return nil
}

func findSelenoidProcesses() []os.Process {
	return findProcesses("selenoid")
}

func findSelenoidUIProcesses() []os.Process {
	return findProcesses("selenoid-ui")
}

func findProcesses(regex string) []os.Process {
	ret := []os.Process{}
	processes, _ := ps.Processes()
	for _, process := range processes {
		matched, _ := regexp.MatchString(regex, process.Executable())
		if matched {
			p, err := os.FindProcess(process.Pid())
			if err == nil {
				ret = append(ret, *p)
			}
		}
	}
	return ret
}

var execCommand = exec.Command

func runCommand(command string, args []string) error {
	cmd := execCommand(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func getSelenoidReleaseFileName() string {
	return getReleaseFileName(selenoidRepo)
}

func getSelenoidUIReleaseFileName() string {
	return getReleaseFileName(selenoidUIRepo)
}

func getReleaseFileName(name string) string {
	rel := fmt.Sprintf("%s_%s_%s", name, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		return rel + ".exe"
	}
	return rel
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return !os.IsNotExist(err)
}
