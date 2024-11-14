package plumber

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config defines the config files for a pipeline.
type Config struct {
	// The repository that should be used for the config files.
	// This should either be the url to a git repo, or a path
	Repo string
	// Revision should be a tag, branch or commit that should be checked out.
	Revision string
	// The local path where the repo should be checked out.
	LocalPath string
}

// Create a new pipeline config.
func NewConfig(repo, revision, path string) Config {
	c := Config{
		Repo:      repo,
		Revision:  revision,
		LocalPath: path,
	}
	return c
}

func ConfigFromPath(path string) (Config, error) {
	var err error
	c := Config{
		LocalPath: path,
	}
	if !c.Exists() {
		return c, fmt.Errorf("directory not found: %s", path)
	}
	c.Revision, err = c.Head()
	if err != nil {
		return c, err
	}
	url, err := c.Remote("origin")
	if err != nil {
		return c, err
	}
	c.Repo = strings.TrimLeft(url.Path, "/")
	return c, nil
}

// Check if the config already exists. Returns true if the local directory exists.
func (c Config) Exists() bool {
	info, err := os.Stat(c.LocalPath)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return false
	}
	return true
}

// Fetch updates from the remote.
func (c Config) Fetch() error {
	if !c.Exists() {
		return fmt.Errorf("directory not found: %s", c.LocalPath)
	}

	cmd := exec.Command("git", "fetch")
	cmd.Dir = c.LocalPath
	slog.Debug("running command", "cmd", cmd.String, "workdir", cmd.Dir)
	return cmd.Run()
}

// Get the checked out tag/branch/commit for the local repository
func (c Config) Head() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = c.LocalPath
	slog.Debug("running command", "cmd", cmd.String(), "workdir", cmd.Dir)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	currentBranch := strings.TrimSpace(string(b))

	if currentBranch != "HEAD" {
		return currentBranch, nil
	}

	cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = c.LocalPath
	slog.Debug("running command", "cmd", cmd.String(), "workdir", cmd.Dir)

	b, err = cmd.Output()
	if err != nil {
		return "", err
	}
	hash := strings.TrimSpace(string(b))

	return hash, cmd.Wait()
}

// Remote returns the remote repository URL for a config.
func (c Config) Remote(name string) (*url.URL, error) {
	cmd := exec.Command("git", "remote", "get-url", name)
	cmd.Dir = c.LocalPath
	stringUrl, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return url.Parse(strings.TrimSpace(string(stringUrl)))
}

// Checkout checks out a specific revision for an existing config.
func (c Config) Checkout(revision string) error {
	if !c.Exists() {
		return fmt.Errorf("directory not found: %s", c.LocalPath)
	}
	cmd := exec.Command("git", "checkout", revision)
	cmd.Dir = c.LocalPath
	slog.Debug("running command", "cmd", cmd.String(), "workdir", cmd.Dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(output)))
	}
	return nil
}

// Clone clones the repository and saves it in the local path.
func (c Config) Clone() error {
	slog.Debug("cloning config file repo")
	argv := []string{
		"clone",
		"-b",
		c.Revision,
		c.Repo,
		c.LocalPath,
	}
	cmd := exec.Command("git", argv...)
	slog.Debug("running command", "cmd", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

// NextflowConfig represents a config for a Nextflow pipeline.
type NextflowConfig struct {
	Config     Config
	Pipeline   Pipeline
	ConfigFile string
	ParamsFile string
	Profile    string
}

// Creates a new nf-core config based on the name of an nf-core pipeline and an existing config.
func NewNextflowConfig(pipeline Pipeline, config Config) (NextflowConfig, error) {
	nextflowConfig := NextflowConfig{Config: config}
	nextflowConfig.Pipeline = pipeline

	if !nextflowConfig.Config.Exists() {
		if err := config.Clone(); err != nil {
			return nextflowConfig, err
		}
	}

	configFile := filepath.Join(nextflowConfig.Config.LocalPath, "nextflow", pipeline.Repo, fmt.Sprintf("%s.config", pipeline.Pipeline))
	paramsFile := filepath.Join(nextflowConfig.Config.LocalPath, "nextflow", pipeline.Repo, "params.yaml")

	if info, err := os.Stat(configFile); err != nil {
		return nextflowConfig, err
	} else if info.IsDir() {
		return nextflowConfig, fmt.Errorf("path is a directory: %s", configFile)
	} else {
		nextflowConfig.ConfigFile = configFile
	}

	if info, err := os.Stat(paramsFile); err == nil && !info.IsDir() {
		nextflowConfig.ParamsFile = paramsFile
	} else {
		slog.Debug("no params file found", "pipeline", pipeline)
	}

	return nextflowConfig, nil
}

type SnakemakeConfig struct {
	Config
}
