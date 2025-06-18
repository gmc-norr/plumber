package plumber

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidPipelineName checks that the name of a pipeline is valid. It should be on the form
// <org>/<repo> and there can be one and only one "/" in the name. Spaces in the organisation
// or pipeline name are not allowed. Returns true if these conditions are fulfilled, otherwise
// false.
func ValidPipelineName(name string) bool {
	frags := strings.Split(name, "/")
	if len(frags) != 2 {
		return false
	}
	if strings.Count(frags[0], " ") != 0 {
		return false
	}
	if strings.Count(frags[1], " ") != 0 {
		return false
	}
	return true
}

// Pipeline represents a pipeline as the name of the repo (<org>/<pipeline>), as well as
// organisation and pipeline separately.
type Pipeline struct {
	Organisation string
	Pipeline     string
	Revision     string
	Repo         string
}

func (p *Pipeline) UnmarshalYAML(value *yaml.Node) error {
	pipeline, err := ParsePipelineName(value.Value)
	*p = pipeline
	return err
}

func (p Pipeline) MarshalYAML() (interface{}, error) {
	return p.Repo, nil
}

// String representation of Pipeline name. The representation is on the form <org>-<repo>, and
// is useful for e.g. paths.
func (n Pipeline) String() string {
	return fmt.Sprintf("%s-%s", n.Organisation, n.Pipeline)
}

// ParsePipelineName parses a repository string (i.e. <org>/<pipeline>) and returns a new
// PipelineName. If the name is invalid or the parsing otherwise fails, error is non-nil.
func ParsePipelineName(name string) (Pipeline, error) {
	pn := Pipeline{Repo: name}
	if !ValidPipelineName(name) {
		return pn, fmt.Errorf("invalid pipeline name")
	}

	nameFrags := strings.Split(name, "/")
	if len(nameFrags) != 2 {
		return pn, fmt.Errorf("expected two fragments, got %d", len(nameFrags))
	}
	pn.Organisation = nameFrags[0]
	pn.Pipeline = nameFrags[1]

	return pn, nil
}

// NextflowPipeline represents a Nextflow pipeline.
type NextflowPipeline struct {
	// The config for the pipeline
	PlumberFile
	Env     map[string]string
	Workdir string
}

// NewNextflowPipeline creates a new Nextflow pipeline.
func NewNextflowPipeline(plumberFile PlumberFile) NextflowPipeline {
	p := NextflowPipeline{
		PlumberFile: plumberFile,
		Env:         make(map[string]string, 0),
	}
	return p
}

// SetEnv sets environment variables that should be accessible when running a Nextflow pipeline.
func (p *NextflowPipeline) SetEnv(key, value string) {
	p.Env[key] = value
}

// Run a nextflow pipeline.
func (p *NextflowPipeline) Run(profile string, extraArgs []string) error {
	slog.Info("starting pipeline execution")
	slog.Debug("settings", "plumber file", p.PlumberFile)
	var args []string
	for _, configFile := range p.ConfigFiles() {
		args = append(args, "-c", configFile)
	}
	for _, profileFile := range p.Profiles() {
		args = append(args, "-c", profileFile)
	}

	args = append(args,
		"run",
		"-ansi-log", "false",
	)

	for _, paramsFile := range p.ParamFiles() {
		args = append(args, "-params-file", paramsFile)
	}
	if profile != "" {
		args = append(args, "-profile")
		args = append(args, profile)
	}
	args = append(args, extraArgs...)
	args = append(args,
		p.PlumberFile.Pipelines[0].Pipeline.Repo,
		"-r",
		p.PlumberFile.Pipelines[0].Version,
	)

	cmd := exec.Command("nextflow", args...)
	cmd.Env = os.Environ()
	for key, value := range p.Env {
		slog.Debug("setting command environment", "key", key, "value", value)
		cmd.Env = append(cmd.Env, fmt.Sprintf(`%s=%s`, key, value))
	}
	if p.Workdir != "" {
		cmd.Dir = p.Workdir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	slog.Debug("running command", "cmd", cmd.String())
	if err := cmd.Start(); err != nil {
		return err
	}

	stdoutScanner := bufio.NewScanner(stdout)
	for stdoutScanner.Scan() {
		fmt.Println(stdoutScanner.Text())
	}

	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		fmt.Println(stderrScanner.Text())
	}

	return cmd.Wait()
}

// Cleanup removes intermediate files from previous executions.
func (p *NextflowPipeline) Cleanup() error {
	workPath := filepath.Join(p.Workdir, "work")
	info, err := os.Stat(workPath)
	if err != nil {
		return fmt.Errorf("error checking work directory %s: %w", workPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", workPath)
	}
	if err := os.RemoveAll(workPath); err != nil {
		return fmt.Errorf("failed to delete work directory: %w", err)
	}
	return nil
}
