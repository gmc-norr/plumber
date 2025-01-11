package plumber

import (
	"bufio"
	"fmt"

	// "io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
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

// CheckPipeline queries the Github API to see whether the pipeline exists.
//
// If the environmet variable PLUMBER_GITHUB_TOKEN is defined, this will be
// used for authentication. If it is missing, and unauthenticated request will
// be made. If the response is http.StatusForbidden, it is probably due to the
// rate limit being exceeded, and the check will return with a warning being
// logged, but without error. If the response is http.StatusUnauthorized, the
// token is invalid and an error will be returned. If the response is
// http.StatusOK, and the revision in question is found, nil will be returned.
func (p Pipeline) Check() error {
	client := &http.Client{}

	slog.Info("checking repo", "name", p.Repo, "revision", p.Revision)

	url := fmt.Sprintf("https://api.github.com/repos/%s", p.Repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if githubToken, ok := os.LookupEnv("PLUMBER_GITHUB_TOKEN"); ok {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", githubToken))
	}
	r, err := client.Do(req)
	if err != nil {
		return err
	}
	if r.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid github token")
	} else if r.StatusCode == http.StatusForbidden {
		slog.Warn("status check forbidden, skipping", "status", r.Status, "tips", "define PLUMBER_GITHUB_TOKEN to increase github API rate limit")
		return nil
	} else if r.StatusCode != http.StatusOK {
		return fmt.Errorf("status: %s", r.Status)
	}

	// Check if rev is a valid commit tag/branch/hash
	commitUrl := url + "/commits/" + p.Revision
	commitReq, err := http.NewRequest("GET", commitUrl, nil)
	if err != nil {
		return err
	}
	commitReq.Header = req.Header
	if r, err := client.Do(commitReq); err == nil && r.StatusCode == http.StatusOK {
		slog.Info("rev is valid", "revision", p.Revision)
		return nil
	} else if r.StatusCode == http.StatusUnprocessableEntity {
		slog.Warn("possibly ambiguous commit hash, try something longer", "hash", p.Revision)
	}

	return fmt.Errorf("revision not found: %s", p.Revision)
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
	args := []string{
		"run",
		"-ansi-log", "false",
		p.PlumberFile.Pipelines[0].Pipeline.Repo,
		"-r",
		p.PlumberFile.Pipelines[0].Pipeline.Revision,
		"-c",
		p.PlumberFile.Pipelines[0].ConfigFiles[0],
	}
	args = append(args, extraArgs...)
	for _, paramsFile := range p.PlumberFile.Pipelines[0].ParamFiles {
		args = append(args, "-params-file")
		args = append(args, paramsFile)
	}
	if profile != "" {
		args = append(args, "-profile")
		args = append(args, profile)
	}
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

	slog.Info("running command", "cmd", cmd.String())
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
