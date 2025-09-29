package plumber

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
		return pn, fmt.Errorf("invalid pipeline name: %q", name)
	}

	nameFrags := strings.Split(name, "/")
	if len(nameFrags) != 2 {
		return pn, fmt.Errorf("expected two fragments, got %d", len(nameFrags))
	}
	pn.Organisation = nameFrags[0]
	pn.Pipeline = nameFrags[1]

	return pn, nil
}

// SnakemakePipeline represents a Snakemake pipeline.
type SnakemakePipeline struct {
	PlumberFile
	Env        map[string]string
	Workdir    string
	Path       string
	Downloaded bool
	Installed  bool
}

// NewSnakemakePipeline creates a new Snakemake pipeline.
func NewSnakemakePipeline(plumberFile PlumberFile) SnakemakePipeline {
	if !plumberFile.singlePipeline() {
		slog.Warn("plumberfile is not for a single pipeline, first pipline definition will be used")
	}
	p := SnakemakePipeline{
		PlumberFile: plumberFile,
		Env:         make(map[string]string, 0),
	}
	for k, v := range p.PlumberFile.Pipelines[0].Environment {
		p.SetEnv(k, v)
	}
	return p
}

func (p *SnakemakePipeline) IsDownloaded() bool {
	if _, err := os.Stat(p.Path); os.IsNotExist(err) {
		return false
	}
	return true
}

func (p *SnakemakePipeline) Download() (err error) {
	if p.IsDownloaded() {
		p.Downloaded = true
		return nil
	}
	if p.Path == "" {
		return fmt.Errorf("pipeline path is missing")
	}
	cmd := exec.Command(
		"git", "clone",
		fmt.Sprintf(
			"https://github.com/%s",
			p.Pipelines[0].Pipeline.Repo,
		),
		p.Path,
	)

	defer func() {
		if err != nil {
			if info, err := os.Stat(p.Path); err == nil && info.IsDir() {
				slog.Info("cleaning up potentially broken pipeline install", "path", p.Path)
				err := os.RemoveAll(p.Path)
				if err != nil {
					slog.Error("failed to clean up pipeline, do it manually", "path", p.Path)
				}
			}
		}
	}()

	slog.Info("downloading pipeline", "cmd", cmd.Args)
	var errBuffer bytes.Buffer
	cmd.Stderr = &errBuffer
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("download failed: %s, %w", errBuffer.String(), err)
	}

	cmd = exec.Command("git", "checkout", p.Pipelines[0].Version)
	errBuffer.Reset()
	cmd.Stderr = &errBuffer
	cmd.Dir = p.Path
	slog.Info("checking out version", "cmd", cmd.Args)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("checkout failed: %s, %w", errBuffer.String(), err)
	}
	p.Downloaded = true
	return nil
}

func (p *SnakemakePipeline) Install() error {
	if !p.Downloaded {
		return fmt.Errorf("pipeline has not been downloaded")
	}
	cmd := exec.Command("python", "-m", "pip", "install", "-r", "requirements.txt")
	cmd.Dir = p.Path
	for k, v := range p.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	slog.Info("installing pipeline", "cmd", cmd.Args, "env", cmd.Env, "workdir", cmd.Dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	outScanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	for outScanner.Scan() {
		slog.Debug("install", "output", outScanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	p.Installed = true
	return nil
}

// SetEnv sets environment variables that should be accessible when running a Nextflow pipeline.
func (p *SnakemakePipeline) SetEnv(key, value string) {
	p.Env[key] = value
}

type LogWriter struct {
	logger *slog.Logger
	level  slog.Level
}

func NewLogWriter(logger *slog.Logger, level slog.Level) LogWriter {
	return LogWriter{
		logger: logger,
		level:  level,
	}
}

func (w LogWriter) Write(b []byte) (n int, err error) {
	for line := range strings.SplitSeq(string(b), "\n") {
		if strings.TrimSpace(line) != "" {
			w.logger.Log(context.TODO(), w.level, line)
		}
	}
	return len(b), nil
}

// Run a Snakemake pipeline
func (p *SnakemakePipeline) Run(profileName string, extraArgs []string) error {
	if profileName == "" {
		return fmt.Errorf("missing profile")
	}
	slog.Debug("starting snakemake execution")
	var availableProfiles []string
	var profileConfig *Profile
	for _, p := range p.Profiles() {
		availableProfiles = append(availableProfiles, p.Name)
		if p.Name == profileName {
			profileConfig = &p
		}
	}
	if profileConfig == nil {
		return fmt.Errorf("profile not found: %s, available profiles: %v", profileName, availableProfiles)
	}
	configs := append(p.ConfigFiles(), profileConfig.ConfigFiles...)
	args := []string{
		"-s", "$PLUMBER_PIPELINE_HOME/workflow/Snakefile",
		"--profile", profileConfig.Profile,
		"--configfiles",
	}
	args = append(args, configs...)
	args = append(args, p.Args()...)
	args = append(args, extraArgs...)

	cmd := exec.Command("snakemake", args...)
	if p.Workdir != "" {
		slog.Debug("setting working directory", "path", p.Workdir)
		cmd.Dir = p.Workdir
	}

	for k, v := range p.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}

	for i, arg := range cmd.Args {
		cmd.Args[i] = os.ExpandEnv(arg)
	}

	cmd.Stderr = NewLogWriter(slog.Default().With("executor", "snakemake", "output", "stderr"), slog.LevelInfo)
	cmd.Stdout = NewLogWriter(slog.Default().With("executor", "snakemake", "output", "stdout"), slog.LevelInfo)

	slog.Debug("executing pipeline", "cmd", cmd.String(), "env", cmd.Env)
	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
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
func (p *NextflowPipeline) Run(profile string, extraArgs []string, webhook *Webhook) error {
	slog.Info("starting pipeline execution")
	slog.Debug("settings", "plumber file", p.PlumberFile)
	var args []string
	for _, configFile := range p.ConfigFiles() {
		args = append(args, "-c", configFile)
	}
	for _, profileFile := range p.Profiles() {
		args = append(args, "-c", profileFile.Path)
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
		p.Pipelines[0].Pipeline.Repo,
		"-r",
		p.Pipelines[0].Version,
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
	go func() {
		for stdoutScanner.Scan() {
			fmt.Println(stdoutScanner.Text())
		}
	}()

	stderrScanner := bufio.NewScanner(stderr)
	go func() {
		for stderrScanner.Scan() {
			fmt.Println(stderrScanner.Text())
		}
	}()

	if webhook != nil {
		slog.Debug("setting up progress messages")
		go func() {
			startTime := time.Now()
			ticker := time.NewTicker(10 * time.Minute)
			msg := WebhookMessage{
				Pipeline:        p.Pipelines[0].Pipeline.String(),
				PipelineVersion: p.Pipelines[0].Version,
				Workdir:         p.Workdir,
				MessageType:     MessageProgress,
			}
			for {
				t := <-ticker.C
				msg.Time = t
				msg.Message = ProgressMessage{
					Message: "execution in progress",
					Elapsed: time.Since(startTime).Round(time.Second).Seconds(),
				}
				if err := webhook.Send(msg); err != nil {
					slog.Error("failed to send progress message to webhook", "error", err)
				}
			}
		}()
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
