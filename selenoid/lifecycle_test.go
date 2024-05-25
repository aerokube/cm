package selenoid

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	assert "github.com/stretchr/testify/require"
)

type MockStrategy struct {
	isDownloaded   bool
	isRunning      bool
	isUIDownloaded bool
	isUIRunning    bool
}

func (ms *MockStrategy) Status() {
	//Does nothing
}

func (ms *MockStrategy) UIStatus() {
	//Does nothing
}

func (ms *MockStrategy) IsDownloaded() bool {
	return ms.isDownloaded
}

func (ms *MockStrategy) IsUIDownloaded() bool {
	return ms.isDownloaded
}

func (ms *MockStrategy) Download() (string, error) {
	return "test", nil
}

func (ms *MockStrategy) DownloadUI() (string, error) {
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

func (ms *MockStrategy) IsUIRunning() bool {
	return ms.isRunning
}

func (ms *MockStrategy) PrintArgs() error {
	return nil
}

func (ms *MockStrategy) PrintUIArgs() error {
	return nil
}

func (ms *MockStrategy) Start() error {
	return nil
}

func (ms *MockStrategy) StartUI() error {
	return nil
}

func (ms *MockStrategy) Stop() error {
	return nil
}

func (ms *MockStrategy) StopUI() error {
	return nil
}

func (ms *MockStrategy) Close() error {
	return nil
}

func TestDockerUnavailable(t *testing.T) {
	assert.False(t, isDockerAvailable())
}

func TestDockerAvailable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mockDockerServer := httptest.NewServer(mux)
	_ = os.Setenv("DOCKER_HOST", "tcp://"+hostPort(mockDockerServer.URL))
	defer os.Unsetenv("DOCKER_HOST")

	assert.True(t, isDockerAvailable())
}

func hostPort(input string) string {
	u, err := url.Parse(input)
	if err != nil {
		panic(err)
	}
	return u.Host
}

func TestLifecycle(t *testing.T) {
	strategy := MockStrategy{}
	lc := createTestLifecycle(strategy)
	defer lc.Close()
	lc.Status()
	assert.NoError(t, lc.Download())
	assert.NoError(t, lc.PrintArgs())
	assert.NoError(t, lc.Configure())
	assert.NoError(t, lc.Start())
	strategy.isRunning = true
	assert.NoError(t, lc.Start())
	strategy.isRunning = false
	assert.NoError(t, lc.Stop())
}

func createTestLifecycle(strategy MockStrategy) Lifecycle {
	return Lifecycle{
		Logger:       Logger{Quiet: false},
		Forceable:    Forceable{Force: true},
		Config:       &LifecycleConfig{},
		argsAware:    &strategy,
		statusAware:  &strategy,
		downloadable: &strategy,
		configurable: &strategy,
		runnable:     &strategy,
		closer:       &strategy,
	}
}

func TestUILifecycle(t *testing.T) {
	strategy := MockStrategy{}
	lc := createTestLifecycle(strategy)
	defer lc.Close()
	lc.UIStatus()
	assert.NoError(t, lc.DownloadUI())
	assert.NoError(t, lc.PrintUIArgs())
	assert.NoError(t, lc.StartUI())
	strategy.isRunning = true
	assert.NoError(t, lc.StartUI())
	strategy.isRunning = false
	assert.NoError(t, lc.StopUI())
}
