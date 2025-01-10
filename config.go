package plumber

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var PlumberFileFormatError = errors.New("invalid formatting of plumber file")

type PlumberFileNotFound struct {
	Path          string
	OriginalError error
}

func (e PlumberFileNotFound) Error() string {
	return fmt.Sprintf("plumber file not found: %s", e.Path)
}

// Config defines the config files for a pipeline.
type Config struct {
	// The repository that should be used for the config files.
	// This should either be the url to a git repo, or a path
	Repo string
	// Version should be a tag, branch or commit that should be checked out.
	Version string
	// The local path where the repo should be checked out.
	LocalPath string
}

const PlumberFileName = "plumber.yaml"

type PlumberFile struct {
	Version   int                      `yaml:"version"`
	Source    string                   `yaml:"source,omitempty"`
	Revision  string                   `yaml:"revision,omitempty"`
	Pipelines []PipelineConfigMetadata `yaml:"pipelines"`
}

func NewPlumberFile() PlumberFile {
	return PlumberFile{
		Version: 1,
	}
}

type PipelineConfigMetadata struct {
	Name        string   `yaml:"name"`
	Engine      string   `yaml:"engine"`
	Version     string   `yaml:"version"`
	Profiles    []string `yaml:"profiles"`
	ConfigFiles []string `yaml:"config_files"`
	ParamFiles  []string `yaml:"param_files"`
	Assets      []string `yaml:"assets"`
}

func (p *PipelineConfigMetadata) AddProfile(path string) {
	p.Profiles = append(p.Profiles, path)
}

func (p *PipelineConfigMetadata) AddConfig(path string) {
	p.ConfigFiles = append(p.ConfigFiles, path)
}

func (p *PipelineConfigMetadata) AddParam(path string) {
	p.ParamFiles = append(p.ParamFiles, path)
}

func (p *PipelineConfigMetadata) AddAssets(path string) {
	p.Assets = append(p.Assets, path)
}

func (p PlumberFile) Validate() error {
	if p.Version != 1 {
		return fmt.Errorf("%w: plumber file version 1 is the only supported version", PlumberFileFormatError)
	}
	for i, p := range p.Pipelines {
		if p.Name == "" {
			return fmt.Errorf("%w: invalid name for pipeline %d: %q", PlumberFileFormatError, i, p.Name)
		}
		if p.Engine == "" {
			return fmt.Errorf("%w: invalid engine for pipeline %d: %q", PlumberFileFormatError, i, p.Engine)
		}
		if p.Version == "" {
			return fmt.Errorf("%w: invalid version for pipeline %d: %q", PlumberFileFormatError, i, p.Version)
		}
		for _, path := range p.ParamFiles {
			if !filepath.IsLocal(path) {
				return fmt.Errorf("%w: param file paths must be relative", PlumberFileFormatError)
			}
		}
		for _, path := range p.ConfigFiles {
			if !filepath.IsLocal(path) {
				return fmt.Errorf("%w: config file paths must be relative", PlumberFileFormatError)
			}
		}
		for _, path := range p.Assets {
			if !filepath.IsLocal(path) {
				return fmt.Errorf("%w: asset paths must be relative", PlumberFileFormatError)
			}
		}
		for _, path := range p.Profiles {
			if !filepath.IsLocal(path) {
				return fmt.Errorf("%w: profile paths must be relative", PlumberFileFormatError)
			}
		}
	}
	return nil
}

func (p PlumberFile) Write(dir string) error {
	if err := p.Validate(); err != nil {
		return err
	}
	b, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	filename := filepath.Join(dir, PlumberFileName)
	return os.WriteFile(filename, b, 0o666)
}

func ReadPlumberFile(directory string) (PlumberFile, error) {
	pf := PlumberFile{}
	path := filepath.Join(directory, PlumberFileName)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return pf, PlumberFileNotFound{
				Path:          path,
				OriginalError: err,
			}
		}
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return pf, err
	}
	err = yaml.Unmarshal(b, &pf)
	if err != nil {
		return pf, PlumberFileFormatError
	}
	return pf, pf.Validate()
}

// Create a new pipeline config.
func NewConfig(repo, branch, path string) Config {
	c := Config{
		Repo:      repo,
		Version:   branch,
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
	_, err = ReadPlumberFile(c.LocalPath)
	if err != nil {
		return c, err
	}
	c.Version, err = c.Head()
	if err != nil {
		return c, err
	}
	url, err := c.Remote("origin")
	if err != nil {
		return c, err
	}
	c.Repo = url.String()
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
	slog.Debug("running command", "cmd", cmd.String(), "workdir", cmd.Dir)
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

	return hash, nil
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
	if err := c.Fetch(); err != nil {
		return err
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

func (c Config) Download(pipeline, version string) (err error) {
	tmpDest, err := os.MkdirTemp(os.TempDir(), "plumber-")
	if err != nil {
		return err
	}
	r := GitRepo{
		Url:       c.Repo,
		LocalPath: tmpDest,
	}
	if err := r.Clone(); err != nil {
		return err
	}

	defer func() {
		slog.Debug("cleaning up cloned directory", "dir", tmpDest)
		if err := os.RemoveAll(tmpDest); err != nil {
			slog.Error("failed to clean up, do it manually", "dir", tmpDest, "error", err)
		}
	}()

	if err := r.Checkout(c.Version); err != nil {
		return err
	}

	pf, err := ReadPlumberFile(tmpDest)
	if err != nil {
		return err
	}

	var pipelineData *PipelineConfigMetadata
	slog.Debug("test", "data", pipelineData)
	nameIdx := -1
	for i, p := range pf.Pipelines {
		if p.Name == pipeline {
			nameIdx = i
			if p.Version == version {
				pipelineData = &p
			}
		}
	}

	if pipelineData == nil {
		msg := "pipeline not found in repo"
		if nameIdx > -1 {
			msg += fmt.Sprintf(", but different version was found: %s", pf.Pipelines[nameIdx].Version)
		}
		return fmt.Errorf("%s", msg)
	}

	slog.Debug("found pipeline", "name", pipeline, "version", version, "d", pipelineData)

	slog.Debug("creating directory", "path", c.LocalPath)
	if err := os.MkdirAll(c.LocalPath, os.ModePerm); err != nil {
		return err
	}

	defer func() {
		if err != nil {
			slog.Debug("cleaning up potentially broken config", "dir", c.LocalPath)
			cleanErr := os.RemoveAll(c.LocalPath)
			if cleanErr != nil {
				slog.Error("failed to clean up config, do it manually", "dir", c.LocalPath)
			}
		}
	}()

	pipelineConfig := PipelineConfigMetadata{}
	pipelineConfig.Name = pipelineData.Name
	pipelineConfig.Engine = pipelineData.Engine
	pipelineConfig.Version = pipelineData.Version

	for _, filename := range pipelineData.ConfigFiles {
		slog.Debug("config file", "path", filename)
		source := filepath.Join(tmpDest, filename)
		target := filepath.Join(c.LocalPath, "config", filepath.Base(filename))
		pipelineConfig.AddConfig(filepath.Join("config", filepath.Base(filename)))
		slog.Debug("copying config file", "source", source, "target", target)
		if err := copy(source, target); err != nil {
			return err
		}
	}

	for _, filename := range pipelineData.ParamFiles {
		slog.Debug("param file", "path", filename)
		source := filepath.Join(tmpDest, filename)
		target := filepath.Join(c.LocalPath, "param", filepath.Base(filename))
		pipelineConfig.AddParam(filepath.Join("param", filepath.Base(filename)))
		slog.Debug("copying param file", "source", source, "target", target)
		if err := copy(source, target); err != nil {
			return err
		}
	}

	for _, filename := range pipelineData.Assets {
		slog.Debug("asset", "path", filename)
		source := filepath.Join(tmpDest, filename)
		target := filepath.Join(c.LocalPath, "assets", filepath.Base(filename))
		pipelineConfig.AddAssets(filepath.Join("assets", filepath.Base(filename)))
		slog.Debug("copying assets", "source", source, "target", target)
		if err := copy(source, target); err != nil {
			return err
		}
	}

	for _, filename := range pipelineData.Profiles {
		slog.Debug("profile", "path", filename)
		source := filepath.Join(tmpDest, filename)
		target := filepath.Join(c.LocalPath, "profile", filepath.Base(filename))
		pipelineConfig.AddProfile(filepath.Join("profile", filepath.Base(filename)))
		slog.Debug("copying profile", "source", source, "target", target)
		if err := copy(source, target); err != nil {
			return err
		}
	}

	pipelinePlumberFile := NewPlumberFile()
	pipelinePlumberFile.Source = r.Url
	pipelinePlumberFile.Revision = c.Version
	pipelinePlumberFile.Pipelines = append(pipelinePlumberFile.Pipelines, pipelineConfig)
	slog.Debug("writing plumber file", "dir", c.LocalPath)
	if err := pipelinePlumberFile.Write(c.LocalPath); err != nil {
		return err
	}

	return nil
}

// copy the source file or directory to target. If the parent directory of target
// doesn't exist, it is created, including any non-existing parent directories.
// If source is a directory, then all contents are copied recursively.
func copy(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(source, target)
	}
	slog.Debug("creating directory", "path", filepath.Dir(target))
	if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
		return err
	}
	return copyFile(source, target)
}

// copyDir copies the source directory to target.
func copyDir(source, target string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}

	slog.Debug("creating directory", "path", filepath.Dir(target))
	if err := os.MkdirAll(target, os.ModePerm); err != nil {
		return err
	}

	for _, entry := range entries {
		entrySource := filepath.Join(source, entry.Name())
		entryTarget := filepath.Join(target, entry.Name())
		if entry.IsDir() {
			if err := copyDir(entrySource, entryTarget); err != nil {
				return err
			}
		} else {
			if err := copyFile(entrySource, entryTarget); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies the source file to target.
func copyFile(source, target string) error {
	tf, err := os.Create(target)
	if err != nil {
		return err
	}
	defer tf.Close()
	sf, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sf.Close()

	_, err = io.Copy(tf, sf)
	if err != nil {
		return err
	}
	return tf.Sync()
}

// Clone clones the repository and saves it in the local path.
func (c Config) Clone() error {
	slog.Debug("cloning config file repo")
	argv := []string{
		"clone",
		c.Repo,
		c.LocalPath,
	}
	cmd := exec.Command("git", argv...)
	slog.Debug("running command", "cmd", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return c.Checkout(c.Version)
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
