package plumber

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMarshalPipeline(t *testing.T) {
	pipeline := "nf-core/raredisease"
	p, _ := ParsePipelineName(pipeline)
	s, _ := yaml.Marshal(p)
	if strings.TrimSpace(string(s)) != pipeline {
		t.Errorf("expected %q, got %q", pipeline, s)
	}
}

func TestUnmarshalPipeline(t *testing.T) {
	pipelineString := "nf-core/raredisease"
	pipelineYaml := []byte(fmt.Sprintf("pipeline: %s", pipelineString))
	var pipeline struct {
		Pipeline Pipeline `yaml:"pipeline"`
	}
	p, _ := ParsePipelineName(pipelineString)
	_ = yaml.Unmarshal(pipelineYaml, &pipeline)
	if p.Repo != pipelineString {
		t.Errorf("expected %s, got %s", pipelineString, p.Repo)
	}
	if p.Organisation != "nf-core" {
		t.Errorf("expected %s, got %s", "nf-core", p.Organisation)
	}
	if p.Pipeline != "raredisease" {
		t.Errorf("expected %s, got %s", "raredisease", p.Pipeline)
	}
}

func TestNextflowCleanup(t *testing.T) {
	var pf PlumberFile
	pipeline := NewNextflowPipeline(pf)
	pipeline.Workdir, _ = os.MkdirTemp(os.TempDir(), "workdirtest")
	workPath := filepath.Join(pipeline.Workdir, "work")
	if err := os.Mkdir(workPath, 0o700); err != nil {
		t.Error("failed to create test work directory")
	}
	if err := os.WriteFile(filepath.Join(workPath, "random.txt"), nil, 0o600); err != nil {
		t.Error("failed to write test file in work directory")
	}
	if err := pipeline.Cleanup(); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(workPath); err == nil {
		t.Error("expected work directory to be gone")
	}
}
