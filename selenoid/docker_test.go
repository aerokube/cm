package selenoid

import (
	"fmt"
	. "github.com/aandryashin/matchers"
	"github.com/aerokube/selenoid/config"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

var (
	mockDockerServer *httptest.Server
	imageName        = selenoidImage
)

func init() {
	mockDockerServer = httptest.NewServer(mux())
	os.Setenv("DOCKER_HOST", "tcp://"+hostPort(mockDockerServer.URL))
	os.Setenv("DOCKER_API_VERSION", "1.29")
}

func mux() http.Handler {
	mux := http.NewServeMux()

	//Docker Registry API mock
	mux.HandleFunc("/v2/", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
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
			output := `
			[{
				"Id": "e90e34656806",
				"Names": [ "selenoid" ],
				"Image": "aerokube/selenoid:latest",
				"ImageID": "e216a057b1cb1efc11f8a268f37ef62083e70b1b38323ba252e25ac88904a7e8",
				"Command": "/usr/bin/selenoid",
				"Created": 1367854154,
				"State": "Exited",
				"Status": "Exit 0",
				"Ports": [
					{
						"PrivatePort": 4444,
						"PublicPort": 4444,
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
			`
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
		}
		c, err := NewDockerConfigurator(&lcConfig)
		AssertThat(t, err, Is{nil})
		defer c.Close()
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
			Image: "selenoid/firefox:46.0",
			Port:  "4444",
			Path:  "/wd/hub",
			Tmpfs: tmpfsMap,
		}
		correctFFBrowsers["45.0"] = &config.Browser{
			Image: "selenoid/firefox:45.0",
			Port:  "4444",
			Path:  "/wd/hub",
			Tmpfs: tmpfsMap,
		}
		AssertThat(t, firefoxVersions, EqualTo{config.Versions{
			Default:  "46.0",
			Versions: correctFFBrowsers,
		}})

		operaVersions, hasPhantomjsKey := cfg["opera"]
		AssertThat(t, hasPhantomjsKey, Is{true})
		AssertThat(t, operaVersions, Is{Not{nil}})
		AssertThat(t, operaVersions.Default, EqualTo{"44.0"})

		correctPhantomjsBrowsers := make(map[string]*config.Browser)
		correctPhantomjsBrowsers["2.1.1"] = &config.Browser{
			Image: "selenoid/opera:44.0",
			Port:  "4444",
			Tmpfs: tmpfsMap,
		}
	})
}

func TestStartStopContainer(t *testing.T) {
	c, err := NewDockerConfigurator(&LifecycleConfig{
		RegistryUrl: mockDockerServer.URL,
	})
	AssertThat(t, err, Is{nil})
	AssertThat(t, c.IsRunning(), Is{true})
	AssertThat(t, c.Start(), Is{nil})
	c.Status()
	AssertThat(t, c.Stop(), Is{nil})
}

func TestDownload(t *testing.T) {
	c, err := NewDockerConfigurator(&LifecycleConfig{
		RegistryUrl: mockDockerServer.URL,
		Quiet:       true,
		Version:     Latest,
	})
	AssertThat(t, c.IsDownloaded(), Is{true})
	AssertThat(t, err, Is{nil})
	ref, err := c.Download()
	AssertThat(t, ref, Not{nil})
	AssertThat(t, err, Is{nil})
}

func TestGetSelenoidImage(t *testing.T) {
	defer func() {
		imageName = selenoidImage
	}()
	c, err := NewDockerConfigurator(&LifecycleConfig{
		RegistryUrl: mockDockerServer.URL,
		Quiet:       true,
		Version:     Latest,
	})
	AssertThat(t, err, Is{nil})
	AssertThat(t, c.getSelenoidImage() == nil, Is{false})
	imageName = "aerokube/selenoid-ui"
	AssertThat(t, c.getSelenoidImage() == nil, Is{true})
}
