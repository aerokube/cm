package selenoid

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aerokube/selenoid/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/heroku/docker-registry-client/registry"
	. "vbom.ml/util/sortorder"
	"sort"
	"log"
	"io/ioutil"
)

const (
	latest   = "latest"
	firefox  = "firefox"
	opera    = "opera"
	tag_1216 = "12.16"
)

type Configurator struct {
	Limit       int
	Verbose     bool
	Pull        bool
	RegistryUrl string
	Tmpfs       int
	docker      *client.Client
	reg         *registry.Registry
}

func (c *Configurator) Init() error {
	if !c.Verbose {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
	err := c.initDockerClient()
	if err != nil {
		return err
	}
	err = c.initRegistryClient()
	if err != nil {
		return err
	}
	return nil
}

func (c *Configurator) initDockerClient() error {
	docker, err := client.NewEnvClient()
	if err != nil {
		return fmt.Errorf("Failed to init Docker client: %v", err)
	}
	c.docker = docker
	return nil
}

func (c *Configurator) initRegistryClient() error {
	reg, err := registry.New(c.RegistryUrl, "", "")
	if err != nil {
		return fmt.Errorf("Docker Registry is not available: %v", err)
	}
	c.reg = reg
	return nil
}

func (c *Configurator) Close() {
	if c.docker != nil {
		c.docker.Close()
	}
}

func (c *Configurator) Configure() (string, error) {
	browsers := c.createConfig()
	data, err := json.MarshalIndent(browsers, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to generate configuration: %v", err)
	}
	return string(data), nil
}

func (c *Configurator) createConfig() map[string]config.Versions {
	supportedBrowsers := c.getSupportedBrowsers()
	browsers := make(map[string]config.Versions)
	for browserName, image := range supportedBrowsers {
		c.printf("Processing browser \"%s\"...\n", browserName)
		tags := c.fetchImageTags(image)
		pulledTags := tags
		if c.Pull {
			pulledTags = c.pullImages(image, tags)
		} else if c.Limit > 0 && c.Limit <= len(tags) {
			pulledTags = tags[:c.Limit]
		}

		if len(pulledTags) > 0 {
			browsers[browserName] = c.createVersions(browserName, image, pulledTags)
		}
	}
	return browsers
}

func (c *Configurator) getSupportedBrowsers() map[string]string {
	supportedBrowsers := make(map[string]string)
	supportedBrowsers["firefox"] = "selenoid/firefox"
	supportedBrowsers["chrome"] = "selenoid/chrome"
	supportedBrowsers["opera"] = "selenoid/opera"
	supportedBrowsers["phantomjs"] = "selenoid/phantomjs"
	return supportedBrowsers
}

func (c *Configurator) printf(format string, v ...interface{}) {
	if c.Verbose {
		fmt.Printf(format, v...)
	}
}

func (c *Configurator) fetchImageTags(image string) []string {
	c.printf("Fetching tags for image \"%s\"...\n", image)
	tags, err := c.reg.Tags(image)
	if err != nil {
		c.printf("Failed to fetch tags for image \"%s\"\n", image)
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
		if tag != latest {
			ret = append(ret, tag)
		}
	}
	return ret
}

func (c *Configurator) createVersions(browserName string, image string, tags []string) config.Versions {
	versions := config.Versions{
		Default:  tags[0],
		Versions: make(map[string]*config.Browser),
	}
	for _, tag := range tags {
		browser := &config.Browser{
			Image: imageWithTag(image, tag),
			Port:  "4444",
			Path:  "/",
		}
		if browserName == firefox || (browserName == opera && tag == tag_1216) {
			browser.Path = "/wd/hub"
		}
		if (c.Tmpfs > 0) {
			tmpfs := make(map[string] string)
			tmpfs["/tmp"] = fmt.Sprintf("size=%dm", c.Tmpfs)
			browser.Tmpfs = tmpfs
		}
		versions.Versions[tag] = browser
	}
	return versions
}

func imageWithTag(image string, tag string) string {
	return fmt.Sprintf("%s:%s", image, tag)
}

func (c *Configurator) pullImages(image string, tags []string) []string {
	pulledTags := []string{}
	ctx := context.Background()
loop:
	for _, tag := range tags {
		ref := imageWithTag(image, tag)
		c.printf("Pulling image \"%s\"...\n", ref)
		resp, err := c.docker.ImagePull(ctx, ref, types.ImagePullOptions{})
		if resp != nil {
			resp.Close()
		}
		if err != nil {
			c.printf("Failed to pull image \"%s\": %v\n", ref, err)
			continue
		}
		pulledTags = append(pulledTags, tag)
		if c.Limit > 0 && len(pulledTags) == c.Limit {
			break loop
		}
	}
	return pulledTags
}
