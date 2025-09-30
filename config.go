package plumber

import (
	"crypto/md5"
	"embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

var ErrPlumberFileFormat = errors.New("invalid formatting of plumber file")

type PlumberFileNotFound struct {
	Path          string
	OriginalError error
}

func (e PlumberFileNotFound) Error() string {
	return fmt.Sprintf("plumber file not found: %s", e.Path)
}

const PlumberFileName = "plumber.yaml"

// PlumberFile represents the root plumberfile structure
type PlumberFile struct {
	Path      string             `yaml:"-"`
	Version   int                `yaml:"version" json:"version"`
	Source    string             `yaml:"source,omitempty" json:"source,omitempty"`
	Revision  string             `yaml:"revision,omitempty" json:"revision,omitempty"`
	Pipelines []PipelineMetadata `yaml:"pipelines" json:"pipelines"`
}

// PipelineMetadata represents a single pipeline configuration
type PipelineMetadata struct {
	Pipeline    Pipeline          `yaml:"name" json:"name"`
	Engine      string            `yaml:"engine" json:"engine"`
	Executor    *Executor         `yaml:"executor,omitempty" json:"executor,omitempty"`
	Version     string            `yaml:"version" json:"version"`
	Environment map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`
	Args        map[string]any    `yaml:"args,omitempty" json:"args,omitempty"`
	Profiles    []Profile         `yaml:"profiles,omitempty" json:"profiles,omitempty"`
	ConfigFiles []string          `yaml:"config_files,omitempty" json:"config_files,omitempty"`
	ParamFiles  []string          `yaml:"param_files,omitempty" json:"param_files,omitempty"`
	Assets      []string          `yaml:"assets,omitempty" json:"assets,omitempty"`
}

// Executor represents the pipeline executor configuration
type Executor struct {
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
}

// Profile represents either a string path (for nextflow) or an object (for snakemake)
type Profile struct {
	// For nextflow profiles (string path)
	Path string `yaml:"-" json:"-"`

	// For snakemake profiles (object)
	Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
	Profile     string   `yaml:"profile,omitempty" json:"profile,omitempty"`
	ConfigFiles []string `yaml:"config_files,omitempty" json:"config_files,omitempty"`
}

// UnmarshalYAML implements custom unmarshaling for Profile to handle both string and object types
func (p *Profile) UnmarshalYAML(value *yaml.Node) error {
	// Try to unmarshal as string first (nextflow profile)
	var path string
	if err := value.Decode(&path); err == nil {
		p.Path = path
		return nil
	}

	// Try to unmarshal as object (snakemake profile)
	type ProfileAlias Profile
	var obj ProfileAlias
	if err := value.Decode(&obj); err != nil {
		return err
	}

	*p = Profile(obj)
	return nil
}

// MarshalYAML implements custom marshaling for Profile
func (p Profile) MarshalYAML() (any, error) {
	slog.Debug("marshaling profile")
	// If Path is set, marshal as string (nextflow profile)
	if p.Path != "" {
		return p.Path, nil
	}

	// Otherwise, marshal as object (snakemake profile)
	type ProfileAlias Profile
	return ProfileAlias(p), nil
}

// IsNextflowProfile returns true if this is a nextflow profile (string path)
func (p *Profile) IsNextflowProfile() bool {
	return p.Path != ""
}

// IsSnakemakeProfile returns true if this is a snakemake profile (object)
func (p *Profile) IsSnakemakeProfile() bool {
	return p.Name != "" || p.Profile != "" || len(p.ConfigFiles) > 0
}

type PipelineExecutor struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

func (p PlumberFile) Exists() bool {
	info, err := os.Stat(p.Path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (p PlumberFile) Validate() error {
	for i, p := range p.Pipelines {
		if p.Pipeline.Repo == "" {
			return fmt.Errorf("%w: invalid name for pipeline %d: %q", ErrPlumberFileFormat, i, p.Pipeline.Repo)
		}
		if p.Engine == "" {
			return fmt.Errorf("%w: invalid engine for pipeline %d: %q", ErrPlumberFileFormat, i, p.Engine)
		}
		if p.Version == "" {
			return fmt.Errorf("%w: invalid version for pipeline %d: %q", ErrPlumberFileFormat, i, p.Version)
		}
		for _, path := range p.ParamFiles {
			if !filepath.IsLocal(path) {
				return fmt.Errorf("%w: param file paths must be relative", ErrPlumberFileFormat)
			}
		}
		for _, path := range p.ConfigFiles {
			if !filepath.IsLocal(path) {
				return fmt.Errorf("%w: config file paths must be relative", ErrPlumberFileFormat)
			}
		}
		for _, path := range p.Assets {
			if !filepath.IsLocal(path) {
				return fmt.Errorf("%w: asset paths must be relative", ErrPlumberFileFormat)
			}
		}
		for _, profile := range p.Profiles {
			if profile.IsNextflowProfile() && !filepath.IsLocal(profile.Path) {
				return fmt.Errorf("%w: profile paths must be relative: %s", ErrPlumberFileFormat, profile.Path)
			}
			if profile.IsSnakemakeProfile() {
				for _, path := range profile.ConfigFiles {
					if !filepath.IsLocal(path) {
						return fmt.Errorf("%w: config file paths must be relative", ErrPlumberFileFormat)
					}
				}
			}
		}
	}
	return nil
}

func (p PlumberFile) Hash() [16]byte {
	if !p.singlePipeline() {
		slog.Warn("not a pipeline-specific config")
	}
	s := fmt.Sprintf("%s-%s-%s-%s", p.Source, p.Revision, p.Pipelines[0].Pipeline.Repo, p.Pipelines[0].Version)
	return md5.Sum([]byte(s))
}

func (p PlumberFile) Write() error {
	slog.Debug("writing plumberfile", "plumberfile", p)
	if err := p.Validate(); err != nil {
		return err
	}
	slog.Debug("validation success")
	if p.Path == "" {
		return fmt.Errorf("plumber file has no path for writing")
	}
	b, err := yaml.Marshal(p)
	if err != nil {
		slog.Debug("marshaling failed")
		return err
	}
	filename := filepath.Join(p.Path, PlumberFileName)
	return os.WriteFile(filename, b, 0o666)
}

func (p PlumberFile) singlePipeline() bool {
	return p.Source != "" && len(p.Pipelines) == 1
}

func (p PlumberFile) ConfigFiles() []string {
	if !p.singlePipeline() {
		slog.Warn("not a pipeline-specific config")
	}
	files := make([]string, len(p.Pipelines[0].ConfigFiles))
	for i, f := range p.Pipelines[0].ConfigFiles {
		files[i] = filepath.Join(p.Path, f)
	}
	return files
}

func (p PlumberFile) ParamFiles() []string {
	if !p.singlePipeline() {
		slog.Warn("not a pipeline-specific config")
	}
	files := make([]string, len(p.Pipelines[0].ParamFiles))
	for i, f := range p.Pipelines[0].ParamFiles {
		files[i] = filepath.Join(p.Path, f)
	}
	return files
}

func (p PlumberFile) Args() []string {
	if !p.singlePipeline() {
		slog.Warn("not a pipeline-specific config")
	}
	var args []string
	for k, v := range p.Pipelines[0].Args {
		args = append(args, "--"+k)
		switch val := v.(type) {
		case string:
			args = append(args, val)
		case []any:
			for _, item := range val {
				switch arg := item.(type) {
				case string:
					args = append(args, arg)
				case float64, float32:
					args = append(args, fmt.Sprintf("%.4f", arg))
				case int:
					args = append(args, fmt.Sprintf("%d", arg))
				default:
					slog.Error("unsupported argument type for %s: %T", k, item)
					continue
				}
			}
		default:
			slog.Error("unsupported argument type", "type", fmt.Sprintf("%T", v), "value", v)
			continue
		}
	}
	slog.Debug("processed args", "args", args)
	return args
}

func (p PlumberFile) Profiles() []Profile {
	if !p.singlePipeline() {
		slog.Warn("not a pipeline-specific config")
	}
	profiles := make([]Profile, len(p.Pipelines[0].Profiles))
	for i, profile := range p.Pipelines[0].Profiles {
		profiles[i] = profile
		if profile.IsNextflowProfile() {
			if !strings.Contains(profile.Path, "$") {
				profiles[i].Path = filepath.Join(p.Path, profile.Path)
			}
		} else if profile.IsSnakemakeProfile() {
			if !strings.Contains(profile.Profile, "$") {
				profiles[i].Profile = filepath.Join(p.Path, profile.Profile)
			}
			for j, filename := range profile.ConfigFiles {
				if !strings.Contains(filename, "$") {
					profiles[i].ConfigFiles[j] = filepath.Join(p.Path, filename)
				}
			}
		}
	}
	return profiles
}

func ParsePlumberFile(data []byte) (PlumberFile, error) {
	var pf PlumberFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return pf, err
	}
	slog.Debug("parsed plumberfile", "result", pf)
	return pf, pf.Validate()
}

func ReadPlumberFile(path string) (PlumberFile, error) {
	pf := PlumberFile{}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return pf, PlumberFileNotFound{
				Path:          path,
				OriginalError: err,
			}
		}
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("failed to close file", "path", path, "error", err)
		}
	}()
	b, err := io.ReadAll(f)
	if err != nil {
		return pf, err
	}
	if err := ValidatePlumberFile(b); err != nil {
		return pf, err
	}

	pf, err = ParsePlumberFile(b)
	pf.Path = filepath.Dir(path)
	return pf, err
}

//go:embed schema/*.schema.json
var schemas embed.FS

const maxSchemaVersion = 1

func ValidatePlumberFile(data []byte) error {
	var y map[string]any
	if err := yaml.Unmarshal(data, &y); err != nil {
		return err
	}

	rawSchemaVersion, ok := y["version"]
	if !ok {
		return fmt.Errorf("%w: missing version in plumberfile", ErrPlumberFileFormat)
	}
	schemaVersion, ok := rawSchemaVersion.(int)
	if !ok {
		return fmt.Errorf("%w: version should be an integer", ErrPlumberFileFormat)
	}

	if schemaVersion > maxSchemaVersion {
		return fmt.Errorf("unsupported plumberfile version %d", schemaVersion)
	}

	compiler := jsonschema.NewCompiler()
	schemaFile, err := schemas.Open(fmt.Sprintf("schema/plumber-v%d.schema.json", schemaVersion))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPlumberFileFormat, err)
	}
	if err := compiler.AddResource(fmt.Sprintf("plumberfile/v%d", schemaVersion), schemaFile); err != nil {
		return fmt.Errorf("%w: %w", ErrPlumberFileFormat, err)
	}
	schema, err := compiler.Compile(fmt.Sprintf("plumberfile/v%d", schemaVersion))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPlumberFileFormat, err)
	}

	err = schema.Validate(y)
	if err != nil {
		return fmt.Errorf("schema validation error: %w, %w", ErrPlumberFileFormat, err)
	}
	return nil
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

	return ReadPlumberFile(filepath.Join(tmpDest, PlumberFileName))
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

	var pipelineData *PipelineMetadata
	slog.Debug("looking for pipeline config", "pipeline", plumberFile.Pipelines[0].Pipeline)
	var nameIdx []int
	for i, p := range pf.Pipelines {
		if p.Pipeline.Repo == plumberFile.Pipelines[0].Pipeline.Repo {
			nameIdx = append(nameIdx, i)
			if p.Version == plumberFile.Pipelines[0].Version {
				pipelineData = &p
			}
		}
	}

	if pipelineData == nil {
		msg := "pipeline not found in repo"
		if len(nameIdx) > 0 {
			var otherVersions []string
			for _, i := range nameIdx {
				otherVersions = append(otherVersions, pf.Pipelines[i].Version)
			}
			if len(otherVersions) == 1 {
				msg += fmt.Sprintf(", but other version was found: %s", otherVersions[0])
			} else {
				msg += fmt.Sprintf(", but other versions were found: %s", strings.Join(otherVersions, ", "))
			}
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

	pipelineConfig := *pipelineData

	for i, filename := range pipelineData.ConfigFiles {
		slog.Debug("config file", "path", filename)
		if strings.Contains(filename, "$") {
			slog.Debug("has environment variable, adding as-is", "path", filename)
			continue
		}
		source := filepath.Join(repo.LocalPath, filename)
		target := filepath.Join(plumberFile.Path, "config", filepath.Base(filename))
		pipelineConfig.ConfigFiles[i] = filepath.Join("config", filepath.Base(filename))
		slog.Debug("copying config file", "source", source, "target", target)
		if err := copy(source, target); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w, are the paths in the plumberfile correct?", err)
			}
			return err
		}
	}

	for i, filename := range pipelineData.ParamFiles {
		slog.Debug("param file", "path", filename)
		if strings.Contains(filename, "$") {
			slog.Debug("has environment variable, adding as-is", "path", filename)
			continue
		}
		source := filepath.Join(repo.LocalPath, filename)
		target := filepath.Join(plumberFile.Path, "param", filepath.Base(filename))
		pipelineConfig.ParamFiles[i] = filepath.Join("param", filepath.Base(filename))
		slog.Debug("copying param file", "source", source, "target", target)
		if err := copy(source, target); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w, are the paths in the plumberfile correct?", err)
			}
			return err
		}
	}

	for i, filename := range pipelineData.Assets {
		slog.Debug("asset", "path", filename)
		if strings.Contains(filename, "$") {
			slog.Debug("has environment variable, adding as-is", "path", filename)
			continue
		}
		source := filepath.Join(repo.LocalPath, filename)
		target := filepath.Join(plumberFile.Path, "assets", filepath.Base(filename))
		pipelineConfig.Assets[i] = filepath.Join("assets", filepath.Base(filename))
		slog.Debug("copying assets", "source", source, "target", target)
		if err := copy(source, target); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w, are the paths in the plumberfile correct?", err)
			}
			return err
		}
	}

	for i, profile := range pipelineData.Profiles {
		slog.Debug("profile", "struct", profile)
		if profile.IsNextflowProfile() {
			if strings.Contains(profile.Path, "$") {
				slog.Debug("has environment variable, adding as-is", "path", profile.Path)
			} else {
				source := filepath.Join(repo.LocalPath, profile.Path)
				target := filepath.Join(plumberFile.Path, "profiles", filepath.Base(profile.Path))
				profile.Path = filepath.Join("profiles", filepath.Base(profile.Path))
				slog.Debug("copying profile", "source", source, "target", target)
				if err := copy(source, target); err != nil {
					return fmt.Errorf("%w, are the paths in the plumberfile correct?", err)
				}
			}
		}
		if profile.IsSnakemakeProfile() {
			if strings.Contains(profile.Profile, "$") {
				slog.Debug("has environment variable, adding as-is", "path", profile.Profile)
			} else {
				source := filepath.Join(repo.LocalPath, profile.Profile)
				target := filepath.Join(plumberFile.Path, "profiles", filepath.Base(profile.Profile))
				profile.Profile = filepath.Join("profiles", filepath.Base(profile.Profile))
				slog.Debug("copying profile", "source", source, "target", target)
				if err := copy(source, target); err != nil {
					return fmt.Errorf("%w, are the paths in the plumberfile correct?", err)
				}
			}
			for j, configPath := range profile.ConfigFiles {
				if strings.Contains(configPath, "$") {
					slog.Debug("has environment variable, adding as-is", "path", configPath)
					continue
				}
				source := filepath.Join(repo.LocalPath, configPath)
				target := filepath.Join(plumberFile.Path, "configs", filepath.Base(profile.ConfigFiles[j]))
				profile.ConfigFiles[j] = filepath.Join("configs", filepath.Base(configPath))
				slog.Debug("copying profile config", "source", source, "target", target)
				if err := copy(source, target); err != nil {
					return fmt.Errorf("%w, are the paths in the plumberfile correct?", err)
				}
			}
		}
		slog.Debug("setting pipeline profile", "profile", profile)
		pipelineConfig.Profiles[i] = profile
	}

	plumberFile.Version = pf.Version
	plumberFile.Source = repo.Url.String()
	plumberFile.Revision = repoVersion
	plumberFile.Pipelines[0] = pipelineConfig
	if err := plumberFile.Write(); err != nil {
		return err
	}

	return nil
}

// copy the source file or directory to target. If the parent directory of target
// doesn't exist, it is created, including any non-existing parent directories.
// If source is a directory, then all contents are copied recursively. If the
// source file contains an environment variable it is ignored.
func copy(source, target string) error {
	if strings.Contains(source, "$") {
		return nil
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(source, target)
	}
	if _, err := os.Stat(filepath.Dir(target)); os.IsNotExist(err) {
		slog.Debug("creating directory", "path", filepath.Dir(target))
		if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
			return err
		}
	}
	return copyFile(source, target)
}

// copyDir copies the source directory to target.
func copyDir(source, target string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}

	var targetDir string
	if info, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			slog.Debug("creating directory", "path", filepath.Dir(target))
			if err := os.MkdirAll(target, os.ModePerm); err != nil {
				return err
			}
		}
		targetDir = target
	} else if info.IsDir() {
		targetDir = target
	} else {
		targetDir = filepath.Dir(target)
	}

	slog.Debug("copyDir", "dir", source, "entries", entries)

	for _, entry := range entries {
		entrySource := filepath.Join(source, entry.Name())
		entryTarget := filepath.Join(targetDir, entry.Name())
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
	slog.Debug("copyFile", "source", source, "target", target)
	tf, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		if err := tf.Close(); err != nil {
			slog.Error("failed to close file", "path", target, "error", err)
		}
	}()
	sf, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() {
		if err := sf.Close(); err != nil {
			slog.Error("failed to close file", "path", source, "error", err)
		}
	}()

	_, err = io.Copy(tf, sf)
	if err != nil {
		return err
	}
	return tf.Sync()
}
