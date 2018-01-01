package selenoid

import (
	"fmt"
	. "github.com/aandryashin/matchers"
	"github.com/aerokube/selenoid/config"
	"github.com/docker/docker/api/types"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

var (
	mockDockerServer *httptest.Server
	imageName        string
	containerName    string
	port             int
)

func init() {
	resetImageName()
	resetContainerName()
	resetPort()
	mockDockerServer = httptest.NewServer(mux())
	os.Setenv("DOCKER_HOST", "tcp://"+hostPort(mockDockerServer.URL))
	os.Setenv("DOCKER_API_VERSION", "1.29")
}

func setImageName(name string) {
	imageName = name
}

func resetImageName() {
	setImageName("docker.io/" + selenoidImage)
}

func setContainerName(name string) {
	containerName = name
}

func resetContainerName() {
	setContainerName(selenoidContainerName)
}

func setPort(p int) {
	port = p
}

func resetPort() {
	setPort(SelenoidDefaultPort)
}

func mux() http.Handler {
	mux := http.NewServeMux()

	//Docker Registry API mock
	mux.HandleFunc("/v2/", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	mux.HandleFunc("/v2/aerokube/selenoid/tags/list", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			fmt.Fprintln(w, `{"name":"selenoid", "tags": ["1.4.0", "1.4.1"]}`)
		},
	))

	mux.HandleFunc("/v2/aerokube/selenoid-ui/tags/list", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			fmt.Fprintln(w, `{"name":"selenoid-ui", "tags": ["1.5.2"]}`)
		},
	))

	mux.HandleFunc("/v2/selenoid/firefox/tags/list", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			fmt.Fprintln(w, `{"name":"firefox", "tags": ["46.0", "45.0", "7.0", "latest"]}`)
		},
	))

	mux.HandleFunc("/v2/selenoid/opera/tags/list", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			fmt.Fprintln(w, `{"name":"opera", "tags": ["44.0", "latest"]}`)
		},
	))

	//Docker API mock
	mux.HandleFunc("/v1.29/images/create", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			output := `{"id": "a86cd3433934", "status": "Downloading layer"}`
			w.Write([]byte(output))
		},
	))
	mux.HandleFunc("/v1.29/images/json", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			output := fmt.Sprintf(`
			[{
			
			    "Id": "sha256:e216a057b1cb1efc11f8a268f37ef62083e70b1b38323ba252e25ac88904a7e8",
			    "ParentId": "",
			    "RepoTags": [ "%s:latest" ],
			    "RepoDigests": [],
			    "Created": 1474925151,
			    "Size": 103579269,
			    "VirtualSize": 103579269,
			    "SharedSize": 0,
			    "Labels": { },
			    "Containers": 2
			
			}]
			`, imageName)
			w.Write([]byte(output))
		},
	))
	mux.HandleFunc("/v1.29/containers/create", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			output := `{"id": "e90e34656806", "warnings": []}`
			w.Write([]byte(output))
		},
	))
	mux.HandleFunc("/v1.29/containers/e90e34656806/start", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	))
	mux.HandleFunc("/v1.29/containers/e90e34656806/stop", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	))
	mux.HandleFunc("/v1.29/containers/e90e34656806", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	))
	mux.HandleFunc("/v1.29/containers/kill", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	))
	mux.HandleFunc("/v1.29/containers/json", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			output := fmt.Sprintf(`
			[{
				"Id": "e90e34656806",
				"Names": [ "%s" ],
				"Image": "%s:latest",
				"ImageID": "e216a057b1cb1efc11f8a268f37ef62083e70b1b38323ba252e25ac88904a7e8",
				"Command": "/usr/bin/some-cmd",
				"Created": 1367854154,
				"State": "Exited",
				"Status": "Exit 0",
				"Ports": [
					{
						"PrivatePort": %d,
						"PublicPort": %d,
						"Type": "tcp"
					}
				],
				"Labels": { },
				"SizeRw": 12288,
				"SizeRootFs": 0,
				"HostConfig": {},
				"NetworkSettings": {},
			    	"Mounts": [ ]
				
			}]
			`, containerName, imageName, port, port)
			w.Write([]byte(output))
		},
	))
	return mux
}

func hostPort(input string) string {
	u, err := url.Parse(input)
	if err != nil {
		panic(err)
	}
	return u.Host
}

func TestImageWithTag(t *testing.T) {
	AssertThat(t, imageWithTag("selenoid/firefox", "tag"), EqualTo{"selenoid/firefox:tag"})
}

func TestFetchImageTags(t *testing.T) {
	lcConfig := LifecycleConfig{
		RegistryUrl: mockDockerServer.URL,
		Quiet:       false,
	}
	c, err := NewDockerConfigurator(&lcConfig)
	AssertThat(t, err, Is{nil})
	defer c.Close()
	tags := c.fetchImageTags("selenoid/firefox")
	AssertThat(t, len(tags), EqualTo{3})
	AssertThat(t, tags[0], EqualTo{"46.0"})
	AssertThat(t, tags[1], EqualTo{"45.0"})
	AssertThat(t, tags[2], EqualTo{"7.0"})
}

func TestPullImages(t *testing.T) {
	lcConfig := LifecycleConfig{
		RegistryUrl: mockDockerServer.URL,
		Quiet:       false,
	}
	c, err := NewDockerConfigurator(&lcConfig)
	AssertThat(t, err, Is{nil})
	defer c.Close()
	tags := c.pullImages("selenoid/firefox", []string{"46.0", "45.0"})
	AssertThat(t, len(tags), EqualTo{2})
	AssertThat(t, tags[0], EqualTo{"46.0"})
	AssertThat(t, tags[1], EqualTo{"45.0"})
}

func TestPreProcessImageTags(t *testing.T) {
	image := "selenoid/firefox"
	browserName := "firefox"
	tags := []string{"33.0", "34.0"}

	c, err := NewDockerConfigurator(&LifecycleConfig{
		RegistryUrl: mockDockerServer.URL,
		VNC:         true,
	})
	AssertThat(t, err, Is{nil})
	imageToProcess, tagsToProcess := c.preProcessImageTags(image, browserName, tags)
	AssertThat(t, imageToProcess, EqualTo{"selenoid/vnc"})
	AssertThat(t, tagsToProcess, EqualTo{[]string{"firefox_33.0", "firefox_34.0"}})
}

func TestConfigureDocker(t *testing.T) {
	testConfigure(t, true)
}

func TestLimitNoPull(t *testing.T) {
	testConfigure(t, false)
}

func testConfigure(t *testing.T, download bool) {
	withTmpDir(t, "test-docker-configure", func(t *testing.T, dir string) {

		lcConfig := LifecycleConfig{
			ConfigDir:    dir,
			RegistryUrl:  mockDockerServer.URL,
			Download:     download,
			Quiet:        false,
			LastVersions: 2,
			Tmpfs:        512,
			Browsers:     "firefox,opera",
			Args:         "-limit 42",
			VNC:          true,
			Env:          testEnv,
			BrowserEnv:   testEnv,
		}
		c, err := NewDockerConfigurator(&lcConfig)
		AssertThat(t, err, Is{nil})
		defer c.Close()
		c.registryHostname = ""
		AssertThat(t, c.IsConfigured(), Is{false})
		cfgPointer, err := (*c).Configure()
		AssertThat(t, err, Is{nil})
		AssertThat(t, cfgPointer, Is{Not{nil}})

		cfg := *cfgPointer
		AssertThat(t, len(cfg), EqualTo{2})

		firefoxVersions, hasFirefoxKey := cfg["firefox"]
		AssertThat(t, hasFirefoxKey, Is{true})
		AssertThat(t, firefoxVersions, Is{Not{nil}})

		tmpfsMap := make(map[string]string)
		tmpfsMap["/tmp"] = "size=512m"

		correctFFBrowsers := make(map[string]*config.Browser)
		correctFFBrowsers["46.0"] = &config.Browser{
			Image: "selenoid/vnc:firefox_46.0",
			Port:  "4444",
			Path:  "/wd/hub",
			Tmpfs: tmpfsMap,
			Env:   []string{testEnv},
		}
		correctFFBrowsers["45.0"] = &config.Browser{
			Image: "selenoid/vnc:firefox_45.0",
			Port:  "4444",
			Path:  "/wd/hub",
			Tmpfs: tmpfsMap,
			Env:   []string{testEnv},
		}
		AssertThat(t, firefoxVersions, EqualTo{config.Versions{
			Default:  "46.0",
			Versions: correctFFBrowsers,
		}})

		operaVersions, hasOperaKey := cfg["opera"]
		AssertThat(t, hasOperaKey, Is{true})
		AssertThat(t, operaVersions, Is{Not{nil}})
		AssertThat(t, operaVersions.Default, EqualTo{"44.0"})

		correctOperaBrowsers := make(map[string]*config.Browser)
		correctOperaBrowsers["44.0"] = &config.Browser{
			Image: "selenoid/vnc:opera_44.0",
			Port:  "4444",
			Path:  "/",
			Tmpfs: tmpfsMap,
			Env:   []string{testEnv},
		}
		AssertThat(t, operaVersions, EqualTo{config.Versions{
			Default:  "44.0",
			Versions: correctOperaBrowsers,
		}})
	})
}

func TestStartStopContainer(t *testing.T) {
	c, err := NewDockerConfigurator(&LifecycleConfig{
		RegistryUrl: mockDockerServer.URL,
		Port:        SelenoidDefaultPort,
	})
	AssertThat(t, err, Is{nil})
	AssertThat(t, c.IsRunning(), Is{true})
	AssertThat(t, c.Start(), Is{nil})
	c.Status()
	AssertThat(t, c.Stop(), Is{nil})
}

func TestStartStopUIContainer(t *testing.T) {
	defer func() {
		resetImageName()
		resetContainerName()
		resetPort()
	}()
	c, err := NewDockerConfigurator(&LifecycleConfig{
		RegistryUrl: mockDockerServer.URL,
		Port:        SelenoidUIDefaultPort,
	})
	AssertThat(t, err, Is{nil})
	setContainerName(selenoidUIContainerName)
	setImageName(selenoidUIImage)
	setPort(SelenoidUIDefaultPort)
	AssertThat(t, c.IsUIRunning(), Is{true})
	AssertThat(t, c.StartUI(), Is{nil})
	c.UIStatus()
	AssertThat(t, c.StopUI(), Is{nil})
}

func TestDownload(t *testing.T) {
	c, err := NewDockerConfigurator(&LifecycleConfig{
		RegistryUrl: mockDockerServer.URL,
		Quiet:       true,
		Version:     Latest,
	})
	AssertThat(t, err, Is{nil})
	c.registryHostname = ""
	AssertThat(t, c.IsDownloaded(), Is{true})
	ref, err := c.Download()
	AssertThat(t, ref, Not{nil})
	AssertThat(t, err, Is{nil})
}

func TestDownloadUI(t *testing.T) {
	defer func() {
		resetImageName()
	}()
	c, err := NewDockerConfigurator(&LifecycleConfig{
		RegistryUrl: mockDockerServer.URL,
		Quiet:       true,
		Version:     Latest,
	})
	setImageName(selenoidUIImage)
	AssertThat(t, err, Is{nil})
	c.registryHostname = ""
	AssertThat(t, c.IsUIDownloaded(), Is{true})
	ref, err := c.DownloadUI()
	AssertThat(t, ref, Not{nil})
	AssertThat(t, err, Is{nil})
}

func TestGetSelenoidImage(t *testing.T) {
	defer func() {
		resetImageName()
	}()
	c, err := NewDockerConfigurator(&LifecycleConfig{
		RegistryUrl: mockDockerServer.URL,
		Quiet:       true,
		Version:     Latest,
	})
	AssertThat(t, err, Is{nil})
	AssertThat(t, c.getSelenoidImage() == nil, Is{false})
	setImageName(selenoidUIImage)
	AssertThat(t, c.getSelenoidImage() == nil, Is{true})
}

func TestFindMatchingImage(t *testing.T) {

	var (
		selenoid141 = types.ImageSummary{
			ID:       "1",
			RepoTags: []string{"aerokube/selenoid:1.4.1"},
			Created:  100,
		}
		selenoid143 = types.ImageSummary{
			ID:       "3",
			RepoTags: []string{"aerokube/selenoid:1.4.3"},
			Created:  300,
		}
	)
	images := []types.ImageSummary{
		selenoid141,
		{
			ID:       "2",
			RepoTags: []string{"aerokube/selenoid-ui:1.5.1"},
			Created:  200, //Intentionally using small timestamps
		},
		selenoid143,
	}

	AssertThat(t, findMatchingImage(images, "unknown-image-name", Latest) == nil, Is{true})
	AssertThat(t, findMatchingImage(images, "aerokube/selenoid", "missing-version") == nil, Is{true})

	foundSelenoid141 := findMatchingImage(images, "aerokube/selenoid", "1.4.1")
	AssertThat(t, foundSelenoid141, Not{nil})
	AssertThat(t, *foundSelenoid141, EqualTo{selenoid141})

	foundSelenoidEmpty := findMatchingImage(images, "aerokube/selenoid", "")
	AssertThat(t, foundSelenoidEmpty, Not{nil})
	AssertThat(t, *foundSelenoidEmpty, EqualTo{selenoid143})

	foundSelenoidLatest := findMatchingImage(images, "aerokube/selenoid", Latest)
	AssertThat(t, foundSelenoidLatest, Not{nil})
	AssertThat(t, *foundSelenoidLatest, EqualTo{selenoid143})
}

func TestIsVideoRecordingSupported(t *testing.T) {
	logger := Logger{}
	AssertThat(t, isVideoRecordingSupported(logger, "wrong-version"), Is{false})
	AssertThat(t, isVideoRecordingSupported(logger, "1.3.9"), Is{false})
	AssertThat(t, isVideoRecordingSupported(logger, "1.4.0"), Is{true})
	AssertThat(t, isVideoRecordingSupported(logger, "1.4.1"), Is{true})
	AssertThat(t, isVideoRecordingSupported(logger, "1.5.0"), Is{true})
	AssertThat(t, isVideoRecordingSupported(logger, "latest"), Is{true})
}

func TestFilterOutLatest(t *testing.T) {
	tags := filterOutLatest([]string{"one", "latest", "latest-release", "two"})
	AssertThat(t, tags, EqualTo{[]string{"one", "two"}})
}

func TestChooseVolumeConfigDir(t *testing.T) {
	dirWithoutVariable := chooseVolumeConfigDir("/some/dir", []string{"one", "two"})
	AssertThat(t, dirWithoutVariable, EqualTo{"/some/dir"})
	os.Setenv("OVERRIDE_HOME", "/test/dir")
	defer os.Unsetenv("OVERRIDE_HOME")
	dir := chooseVolumeConfigDir("/some/dir", []string{"one", "two"})
	AssertThat(t, dir, EqualTo{"/test/dir/one/two"})
}

func TestPostProcessPath(t *testing.T) {
	AssertThat(t, postProcessPath("C:\\Users\\admin"), EqualTo{"/c/Users/admin"})
	AssertThat(t, postProcessPath("C:\\C:\\Users\\admin"), EqualTo{"/c/C:/Users/admin"})
	AssertThat(t, postProcessPath("1"), EqualTo{"1"})
	AssertThat(t, postProcessPath(""), EqualTo{""})
}

func TestValidEnviron(t *testing.T) {
	AssertThat(t, validateEnviron([]string{"=::=::"}), EqualTo{[]string{}})
	AssertThat(t, validateEnviron([]string{"HOMEDRIVE=C:", "DOCKER_HOST=192.168.0.1", "=::=::"}), EqualTo{[]string{"HOMEDRIVE=C:", "DOCKER_HOST=192.168.0.1"}})
}
