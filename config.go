package plumber

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

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
	Path      string                   `yaml:"-"`
}

func NewPlumberFile() PlumberFile {
	return PlumberFile{
		Version: 1,
	}
}

func (p PlumberFile) Exists() bool {
	info, err := os.Stat(p.Path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

type PipelineConfigMetadata struct {
	Pipeline    Pipeline `yaml:"name"`
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
	if len(p.Pipelines) == 0 {
		return fmt.Errorf("%w: plumber file does not have any pipeline definitions", PlumberFileFormatError)
	}
	for i, p := range p.Pipelines {
		if p.Pipeline.Repo == "" {
			return fmt.Errorf("%w: invalid name for pipeline %d: %q", PlumberFileFormatError, i, p.Pipeline.Repo)
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

func (p PlumberFile) Write() error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.Path == "" {
		return fmt.Errorf("plumber file has no path for writing")
	}
	b, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	filename := filepath.Join(p.Path, PlumberFileName)
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
		return pf, fmt.Errorf("%w: %w", PlumberFileFormatError, err)
	}
	pf.Path = directory
	return pf, pf.Validate()
}

// DownloadConfigRepo clones a config repo into a temporary directory and sets
// GitRepo.Path accordingly. It returns a PlumberFile, and if the repo
// cannot be cloned, checked out, or the plumber file cannot be read, an
// error is also returned, otherwise nil.
func DownloadConfigRepo(repo *GitRepo, repoVersion string) (PlumberFile, error) {
	var pf PlumberFile
	tmpDest, err := os.MkdirTemp(os.TempDir(), "plumber-")
	if err != nil {
		return pf, err
	}
	repo.LocalPath = tmpDest

	if err := repo.Clone(); err != nil {
		return pf, err
	}

	if err := repo.Checkout(repoVersion); err != nil {
		return pf, err
	}

	return ReadPlumberFile(tmpDest)
}

func DownloadConfig(repo GitRepo, repoVersion string, plumberFile *PlumberFile) (err error) {
	pf, err := DownloadConfigRepo(&repo, repoVersion)
	if err != nil {
		return err
	}

	defer func() {
		slog.Debug("cleaning up cloned directory", "dir", repo.LocalPath)
		if err := os.RemoveAll(repo.LocalPath); err != nil {
			slog.Error("failed to clean up, do it manually", "dir", repo.LocalPath, "error", err)
		}
	}()

	var pipelineData *PipelineConfigMetadata
	nameIdx := -1
	for i, p := range pf.Pipelines {
		if p.Pipeline == plumberFile.Pipelines[0].Pipeline {
			nameIdx = i
			if p.Version == plumberFile.Pipelines[0].Version {
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

	slog.Debug("found pipeline", "name", plumberFile.Pipelines[0].Pipeline, "version", plumberFile.Pipelines[0].Version, "d", pipelineData)

	slog.Debug("creating directory", "path", plumberFile.Path)
	if err := os.MkdirAll(plumberFile.Path, os.ModePerm); err != nil {
		return err
	}

	defer func() {
		if err != nil {
			slog.Debug("cleaning up potentially broken config", "dir", plumberFile.Path)
			cleanErr := os.RemoveAll(plumberFile.Path)
			if cleanErr != nil {
				slog.Error("failed to clean up config, do it manually", "dir", plumberFile.Path)
			}
		}
	}()

	pipelineConfig := PipelineConfigMetadata{}
	pipelineConfig.Pipeline = pipelineData.Pipeline
	pipelineConfig.Engine = pipelineData.Engine
	pipelineConfig.Version = pipelineData.Version

	for _, filename := range pipelineData.ConfigFiles {
		slog.Debug("config file", "path", filename)
		source := filepath.Join(repo.LocalPath, filename)
		target := filepath.Join(plumberFile.Path, "config", filepath.Base(filename))
		pipelineConfig.AddConfig(filepath.Join("config", filepath.Base(filename)))
		slog.Debug("copying config file", "source", source, "target", target)
		if err := copy(source, target); err != nil {
			return err
		}
	}

	for _, filename := range pipelineData.ParamFiles {
		slog.Debug("param file", "path", filename)
		source := filepath.Join(repo.LocalPath, filename)
		target := filepath.Join(plumberFile.Path, "param", filepath.Base(filename))
		pipelineConfig.AddParam(filepath.Join("param", filepath.Base(filename)))
		slog.Debug("copying param file", "source", source, "target", target)
		if err := copy(source, target); err != nil {
			return err
		}
	}

	for _, filename := range pipelineData.Assets {
		slog.Debug("asset", "path", filename)
		source := filepath.Join(repo.LocalPath, filename)
		target := filepath.Join(plumberFile.Path, "assets", filepath.Base(filename))
		pipelineConfig.AddAssets(filepath.Join("assets", filepath.Base(filename)))
		slog.Debug("copying assets", "source", source, "target", target)
		if err := copy(source, target); err != nil {
			return err
		}
	}

	for _, filename := range pipelineData.Profiles {
		slog.Debug("profile", "path", filename)
		source := filepath.Join(repo.LocalPath, filename)
		target := filepath.Join(plumberFile.Path, "profile", filepath.Base(filename))
		pipelineConfig.AddProfile(filepath.Join("profile", filepath.Base(filename)))
		slog.Debug("copying profile", "source", source, "target", target)
		if err := copy(source, target); err != nil {
			return err
		}
	}

	plumberFile.Pipelines[0] = pipelineConfig
	slog.Debug("writing plumber file", "dir", plumberFile.Path)
	if err := plumberFile.Write(); err != nil {
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
