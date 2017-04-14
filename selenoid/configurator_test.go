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
	mock *httptest.Server
)

func init() {
	mock = httptest.NewServer(mux())
	os.Setenv("DOCKER_HOST", "tcp://"+hostPort(mock.URL))
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

	mux.HandleFunc("/v2/selenoid/phantomjs/tags/list", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			fmt.Fprintln(w, `{"name":"phantomjs", "tags": ["2.1.1", "latest"]}`)
		},
	))

	//Docker API mock
	mux.HandleFunc("/v1.29/images/create", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
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
	c, err := NewConfigurator(mock.URL, true)
	AssertThat(t, err, Is{nil})
	defer c.Close()
	tags := c.fetchImageTags("selenoid/firefox")
	AssertThat(t, len(tags), EqualTo{3})
	AssertThat(t, tags[0], EqualTo{"46.0"})
	AssertThat(t, tags[1], EqualTo{"45.0"})
	AssertThat(t, tags[2], EqualTo{"7.0"})
}

func TestPullImages(t *testing.T) {
	c, err := NewConfigurator(mock.URL, true)
	AssertThat(t, err, Is{nil})
	defer c.Close()
	tags := c.pullImages("selenoid/firefox", []string{"46.0", "45.0"})
	AssertThat(t, len(tags), EqualTo{2})
	AssertThat(t, tags[0], EqualTo{"46.0"})
	AssertThat(t, tags[1], EqualTo{"45.0"})
}

func TestCreateConfig(t *testing.T) {
	testCreateConfig(t, true)
}

func TestLimitNoPull(t *testing.T) {
	testCreateConfig(t, false)
}

func testCreateConfig(t *testing.T, pull bool) {
	c, err := NewConfigurator(mock.URL, true)
	AssertThat(t, err, Is{nil})
	c.LastVersions = 2
	c.Pull = pull
	c.Tmpfs = 512
	defer c.Close()
	cfg := c.createConfig()
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

	phantomjsVersions, hasPhantomjsKey := cfg["phantomjs"]
	AssertThat(t, hasPhantomjsKey, Is{true})
	AssertThat(t, phantomjsVersions, Is{Not{nil}})
	AssertThat(t, phantomjsVersions.Default, EqualTo{"2.1.1"})

	correctPhantomjsBrowsers := make(map[string]*config.Browser)
	correctPhantomjsBrowsers["2.1.1"] = &config.Browser{
		Image: "selenoid/phantomjs:2.1.1",
		Port:  "4444",
		Tmpfs: tmpfsMap,
	}
}
