package selenoid

import (
	. "github.com/aandryashin/matchers"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type MockStrategy struct {
	isDownloaded bool
	isRunning    bool
}

func (ms *MockStrategy) Status() {
	//Does nothing
}

func (ms *MockStrategy) IsDownloaded() bool {
	return ms.isDownloaded
}

func (ms *MockStrategy) Download() (string, error) {
	return "test", nil
}

func (ms *MockStrategy) IsConfigured() bool {
	return false
}

func (ms *MockStrategy) Configure() (*SelenoidConfig, error) {
	return &SelenoidConfig{}, nil
}

func (ms *MockStrategy) IsRunning() bool {
	return ms.isRunning
}

func (ms *MockStrategy) Start() error {
	return nil
}

func (ms *MockStrategy) Stop() error {
	return nil
}

func (ms *MockStrategy) Close() error {
	return nil
}

func TestDockerUnavailable(t *testing.T) {
	AssertThat(t, isDockerAvailable(), Is{false})
}

func TestDockerAvailable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))
	mockDockerServer := httptest.NewServer(mux)
	os.Setenv("DOCKER_HOST", "tcp://"+hostPort(mockDockerServer.URL))
	os.Setenv("DOCKER_API_VERSION", "1.29")

	AssertThat(t, isDockerAvailable(), Is{true})

	os.Unsetenv("DOCKER_HOST")
	os.Unsetenv("DOCKER_API_VERSION")
}

func TestLifecycle(t *testing.T) {
	strategy := MockStrategy{}
	lc := Lifecycle{
		Logger:       Logger{Quiet: false},
		Forceable:    Forceable{Force: true},
		Config:       &LifecycleConfig{},
		statusAware:  &strategy,
		downloadable: &strategy,
		configurable: &strategy,
		runnable:     &strategy,
		closer:       &strategy,
	}
	defer lc.Close()
	lc.Status()
	AssertThat(t, lc.Download(), Is{nil})
	AssertThat(t, lc.Configure(), Is{nil})
	AssertThat(t, lc.Start(), Is{nil})
	strategy.isRunning = true
	AssertThat(t, lc.Start(), Is{nil})
	strategy.isRunning = false
	AssertThat(t, lc.Stop(), Is{nil})
}
