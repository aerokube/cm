package selenoid

import (
	"encoding/json"
	"fmt"
	"github.com/aerokube/selenoid/config"
	"github.com/google/go-github/github"
	assert "github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path"
	"reflect"
	"runtime"
	"testing"
	"time"
)

const (
	previousReleaseTag = "1.2.0"
	latestReleaseTag   = "1.2.1"
	version            = "version"
	testEnv            = "MYKEY=myvalue"
)

var (
	mockDriverServer *httptest.Server
	releaseFileName  = getSelenoidReleaseFileName()
)

func init() {
	mockDriverServer = httptest.NewServer(driversMux())
	killFunc = func(_ *os.Process, _ bool, _ time.Duration) error { return nil }
}

func driversMux() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/browsers.json", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			goos := runtime.GOOS
			goarch := runtime.GOARCH
			browsers := Browsers{
				"first": Browser{
					Command: "%s",
					Files: Files{
						goos: {
							goarch: Driver{
								URL:      mockServerUrl(mockDriverServer, "/testfile.zip"),
								Filename: "zip-testfile",
							},
						},
					},
				},
				"second": Browser{
					Command: "%s",
					Files: Files{
						goos: {
							goarch: Driver{
								URL:      mockServerUrl(mockDriverServer, "/testfile.tar.gz"),
								Filename: "gzip-testfile",
							},
						},
					},
				},
				"MicrosoftEdge": Browser{
					Command: "%s",
					Files: Files{
						goos: {
							goarch: Driver{
								URL:      mockServerUrl(mockDriverServer, "/testfile"),
								Filename: "testfile",
							},
						},
					},
				},
				"safari": Browser{
					Command: "%s",
					Files: Files{
						goos: {
							goarch: Driver{
								URL:      "",
								Filename: "/usr/bin/safaridriver",
							},
						},
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(&browsers)
		},
	))

	mux.HandleFunc(
		fmt.Sprintf("/repos/%s/%s/releases/tags/%s", owner, selenoidRepo, previousReleaseTag),
		http.HandlerFunc(getReleaseHandler(previousReleaseTag)),
	)
	mux.HandleFunc(
		fmt.Sprintf("/repos/%s/%s/releases/latest", owner, selenoidRepo),
		http.HandlerFunc(getReleaseHandler(latestReleaseTag)),
	)
	mux.HandleFunc(
		fmt.Sprintf("/repos/%s/%s/releases/tags/%s", owner, selenoidUIRepo, previousReleaseTag),
		http.HandlerFunc(getReleaseHandler(previousReleaseTag)),
	)
	mux.HandleFunc(
		fmt.Sprintf("/repos/%s/%s/releases/latest", owner, selenoidUIRepo),
		http.HandlerFunc(getReleaseHandler(latestReleaseTag)),
	)
	mux.HandleFunc("/"+releaseFileName, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			version := r.URL.Query().Get(version)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(version))
		},
	))

	//Serving static files from current directory
	mux.Handle("/", http.FileServer(http.Dir("")))

	return mux
}

func TestAllUrlsAreValid(t *testing.T) {

	dir, err := os.Getwd()
	assert.NoError(t, err)

	data := readFile(t, path.Join(dir, "..", "browsers.json"))

	var browsers Browsers
	err = json.Unmarshal(data, &browsers)
	assert.NoError(t, err)

	//Loops are ugly but we need to check all urls in one test...
	for _, browser := range browsers {
		for _, architectures := range browser.Files {
			for _, driver := range architectures {
				u := driver.URL
				if u != "" {
					fmt.Printf("Checking URL: %s\n", u)
					req, err := http.NewRequest(http.MethodHead, u, nil)
					client := &http.Client{
						CheckRedirect: func(req *http.Request, via []*http.Request) error {
							/*
								Do not follow redirects in order to avoid 403 Forbidden responses from S3 when checking Github releases links
							*/
							return http.ErrUseLastResponse
						},
					}
					resp, err := client.Do(req)
					if err != nil {
						t.Fatalf("failed to request url %s: %v\n", u, err)
					}
					if resp.StatusCode != 200 && resp.StatusCode != 301 && resp.StatusCode != 302 {
						t.Fatalf("broken url %s: %d", u, resp.StatusCode)
					}
				}
			}
		}
	}
}

func TestConfigureDrivers(t *testing.T) {

	withTmpDir(t, "test-download", func(t *testing.T, dir string) {
		driversInfoUrl := mockServerUrl(mockDriverServer, "/browsers.json")
		lcConfig := LifecycleConfig{
			ConfigDir:      dir,
			Browsers:       "first;second;safari;fourth",
			DriversInfoUrl: driversInfoUrl,
			Download:       true,
			Quiet:          false,
			Args:           "-limit 42",
			Env:            testEnv,
			BrowserEnv:     testEnv,
		}
		configurator := NewDriversConfigurator(&lcConfig)
		assert.False(t, configurator.IsConfigured())
		cfgPointer, err := (*configurator).Configure()
		assert.NoError(t, err)
		assert.NotNil(t, cfgPointer)

		cfg := *cfgPointer
		assert.Len(t, cfg, 3)

		unpackedFirstFile := path.Join(dir, "zip-testfile")
		unpackedSecondFile := path.Join(dir, "gzip-testfile")
		correctConfig := SelenoidConfig{
			"first": config.Versions{
				Default: Latest,
				Versions: map[string]*config.Browser{
					Latest: {
						Image: []string{unpackedFirstFile},
						Path:  "/",
						Env:   []string{testEnv},
					},
				},
			},
			"second": config.Versions{
				Default: Latest,
				Versions: map[string]*config.Browser{
					Latest: {
						Image: []string{unpackedSecondFile},
						Path:  "/",
						Env:   []string{testEnv},
					},
				},
			},
			"safari": config.Versions{
				Default: Latest,
				Versions: map[string]*config.Browser{
					Latest: {
						Image: []string{"/usr/bin/safaridriver"},
						Path:  "/",
						Env:   []string{testEnv},
					},
				},
			},
		}

		if !reflect.DeepEqual(cfg, correctConfig) {
			cfgData, _ := json.MarshalIndent(cfg, "", "    ")
			correctConfigData, _ := json.MarshalIndent(correctConfig, "", "    ")
			t.Fatalf("Incorrect config. Expected:\n %+v\n Actual: %+v\n", string(correctConfigData), string(cfgData))
		}

		for _, unpackedFile := range []string{unpackedFirstFile, unpackedSecondFile} {
			if !fileExists(unpackedFile) {
				t.Fatalf("file %s does not exist\n", unpackedFile)
			}
		}
	})

}

func TestUnzip(t *testing.T) {
	data := readFile(t, "testfile.zip")
	assert.True(t, isZipFile(data))
	assert.False(t, isTarGzFile(data))
	testUnpack(t, data, "zip-testfile", func(data []byte, filePath string, outputDir string) (string, error) {
		return unzip(data, filePath, outputDir)
	}, "zip\n")
}

func TestUntar(t *testing.T) {
	data := readFile(t, "testfile.tar.gz")
	assert.True(t, isTarGzFile(data))
	assert.False(t, isZipFile(data))
	testUnpack(t, data, "gzip-testfile", func(data []byte, filePath string, outputDir string) (string, error) {
		return untar(data, filePath, outputDir)
	}, "gzip\n")
}

func testUnpack(t *testing.T, data []byte, fileName string, fn func([]byte, string, string) (string, error), correctContents string) {

	withTmpDir(t, "test-unpack", func(t *testing.T, dir string) {
		unpackedFile, err := fn(data, fileName, dir)
		if err != nil {
			t.Fatal(err)
		}

		if !fileExists(unpackedFile) {
			t.Fatalf("file %s does not exist\n", unpackedFile)
		}

		unpackedFileContents := string(readFile(t, unpackedFile))
		if unpackedFileContents != correctContents {
			t.Fatalf("incorrect unpacked file contents; expected: '%s', actual: '%s'\n", correctContents, unpackedFileContents)
		}
	})

}

func readFile(t *testing.T, fileName string) []byte {
	data, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDownloadFile(t *testing.T) {
	fileUrl := mockServerUrl(mockDriverServer, "/testfile")
	data, err := downloadFile(fileUrl)
	if err != nil {
		t.Fatalf("failed to download file: %v\n", err)
	}
	assert.Equal(t, string(data), "test-data")
}

func mockServerUrl(mockServer *httptest.Server, relativeUrl string) string {
	base, _ := url.Parse(mockServer.URL)
	relative, _ := url.Parse(relativeUrl)
	return base.ResolveReference(relative).String()
}

func withTmpDir(t *testing.T, prefix string, fn func(*testing.T, string)) {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	fn(t, dir)

}

func getReleaseHandler(v string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		releaseUrl := mockServerUrl(
			mockDriverServer,
			fmt.Sprintf("/%s?%s=%s", releaseFileName, version, v),
		)
		release := github.RepositoryRelease{
			Assets: []github.ReleaseAsset{
				{
					Name:               &releaseFileName,
					BrowserDownloadURL: &releaseUrl,
				},
			},
		}
		data, _ := json.Marshal(&release)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}
}

func TestDownloadLatestRelease(t *testing.T) {
	testDownloadRelease(t, Latest, latestReleaseTag)
}

func TestDownloadSpecificRelease(t *testing.T) {
	testDownloadRelease(t, previousReleaseTag, previousReleaseTag)
}

func testDownloadRelease(t *testing.T, desiredVersion string, expectedFileContents string) {
	withTmpDir(t, "downloader", func(t *testing.T, dir string) {
		lcConfig := LifecycleConfig{
			GithubBaseUrl: mockDriverServer.URL + "/",
			ConfigDir:     dir,
			OS:            runtime.GOOS,
			Arch:          runtime.GOARCH,
			Version:       desiredVersion,
		}
		configurator := NewDriversConfigurator(&lcConfig)
		assert.False(t, configurator.IsDownloaded())

		outputPath, err := configurator.Download()
		assert.NoError(t, err)
		assert.NotNil(t, outputPath)
		checkContentsEqual(t, outputPath, expectedFileContents)

		uiOutputPath, err := configurator.DownloadUI()
		assert.NoError(t, err)
		assert.NotNil(t, uiOutputPath)
		checkContentsEqual(t, uiOutputPath, expectedFileContents)
	})

}

func checkContentsEqual(t *testing.T, outputPath string, expectedFileContents string) {
	if !fileExists(outputPath) {
		t.Fatalf("release was not downloaded to %s: file does not exist\n", outputPath)
	}
	data, err := os.ReadFile(outputPath)
	assert.NoError(t, err)
	assert.Equal(t, string(data), expectedFileContents)

}

func TestUnknownRelease(t *testing.T) {
	downloadShouldFail(t, func(dir string) *DriversConfigurator {
		lcConfig := LifecycleConfig{
			GithubBaseUrl: mockDriverServer.URL,
			ConfigDir:     dir,
			OS:            runtime.GOOS,
			Arch:          runtime.GOARCH,
			Version:       "missing-version",
		}
		return NewDriversConfigurator(&lcConfig)
	})
}

func downloadShouldFail(t *testing.T, fn func(string) *DriversConfigurator) {
	withTmpDir(t, "something", func(t *testing.T, dir string) {
		configurator := fn(dir)
		_, err := configurator.Download()
		assert.Error(t, err)
	})
}

func TestUnavailableBinary(t *testing.T) {
	downloadShouldFail(t, func(dir string) *DriversConfigurator {
		lcConfig := LifecycleConfig{
			GithubBaseUrl: mockDriverServer.URL,
			ConfigDir:     dir,
			OS:            "missing-os",
			Arch:          "missing-arch",
			Version:       previousReleaseTag,
		}
		return NewDriversConfigurator(&lcConfig)
	})
}

func TestWrongBaseUrl(t *testing.T) {
	downloadShouldFail(t, func(dir string) *DriversConfigurator {
		lcConfig := LifecycleConfig{
			GithubBaseUrl: ":::bad-url:::",
			ConfigDir:     dir,
			OS:            runtime.GOOS,
			Arch:          runtime.GOARCH,
			Version:       Latest,
		}
		return NewDriversConfigurator(&lcConfig)
	})
}

// Based on https://npf.io/2015/06/testing-exec-command/
func TestStartStopProcess(t *testing.T) {
	execCommand = fakeExecCommand
	defer func() {
		execCommand = exec.Command
	}()
	withTmpDir(t, "something", func(t *testing.T, dir string) {
		lcConfig := LifecycleConfig{
			GithubBaseUrl: mockDriverServer.URL,
			ConfigDir:     dir,
			OS:            runtime.GOOS,
			Arch:          runtime.GOARCH,
			Version:       Latest,
			Port:          SelenoidDefaultPort,
		}
		configurator := NewDriversConfigurator(&lcConfig)
		assert.True(t, configurator.IsRunning()) //This is probably true because test binary has name selenoid.test; no fake process is launched
		assert.NoError(t, configurator.Start())
		configurator.Status()
		assert.NoError(t, configurator.Stop())
		assert.NoError(t, configurator.PrintArgs())

		lcConfig.Port = SelenoidUIDefaultPort
		assert.False(t, configurator.IsUIRunning())
		assert.NoError(t, configurator.StartUI())
		configurator.UIStatus()
		assert.NoError(t, configurator.StopUI())
		assert.NoError(t, configurator.PrintUIArgs())
	})

}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestPrepareCommand(t *testing.T) {
	assert.Equal(
		t,
		prepareCommand("%s --some-arg", "/path/with spaces"),
		[]string{
			"/path/with spaces", "--some-arg",
		},
	)
}
