package plumber

import (
	"fmt"
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
