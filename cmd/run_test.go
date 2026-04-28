package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gmc-norr/plumber"
	// "github.com/maehler/webhook"
	"github.com/spf13/cobra"
)

func runInTempdir(t *testing.T, cmd *cobra.Command, args []string) (string, string, string, error) {
	tmp := t.TempDir()
	args = append(args, "-d", tmp)
	cmd.SetArgs(args)

	var outbuf, errbuf bytes.Buffer
	cmd.SetOut(&outbuf)
	cmd.SetErr(&errbuf)

	t.Logf("cmdline: %v", args)

	err := cmd.Execute()

	return tmp, outbuf.String(), errbuf.String(), err
}

// Test that `nextflow run` exists
func TestNextflowRun(t *testing.T) {
	cmd := newTestRootCmd(t)
	_, _, _, err := runInTempdir(t, cmd, []string{"nextflow", "run", "--help"})
	if err != nil {
		t.Error(err)
	}
}

// Test that nextflow run outputs `.plumber-analysis.json`
func TestNextflowRunAnalysisFile(t *testing.T) {
	v := newTestViper(t)
	cmd := NewRootCmd(v)
	wd, _, _, err := runInTempdir(t, cmd, []string{"nextflow", "run", "nf-core/raredisease", "--version", "2.4.0", "-l", "debug"})
	if err == nil {
		t.Error("expected execution to fail but it didn't")
	}

	_, err = findPlumberFile(v)
	if err != nil {
		t.Error(err)
	}

	analysisFilePath := filepath.Join(wd, `.plumber-analysis.json`)
	if _, err := os.Stat(analysisFilePath); err != nil {
		t.Error(err)
	}
}

// Test that `hydra run` exists
func TestHydraRun(t *testing.T) {
	cmd := newTestRootCmd(t)
	_, _, _, err := runInTempdir(t, cmd, []string{"hydra", "run", "--help"})
	if err != nil {
		t.Error(err)
	}
}

// Test that `hydra run` downloads the config and outputs `.plumber-analysis.json`
func TestHydraRunAnalyisFile(t *testing.T) {
	v := newTestViper(t)
	cmd := NewRootCmd(v)
	wd, _, _, err := runInTempdir(t, cmd, []string{"hydra", "run", "genomic-medicine-sweden/Twist_Solid", "--version", "v0.23.0", "-l", "debug", "--profile", "slurm_hg19_research"})
	if err == nil {
		t.Error("expected execution to fail but it didn't")
	}

	_, err = findPlumberFile(v)
	if err != nil {
		t.Error(err)
	}

	analysisFilePath := filepath.Join(wd, `.plumber-analysis.json`)
	if _, err := os.Stat(analysisFilePath); err != nil {
		t.Error(err)
	}

	if _, err := os.Stat(analysisFilePath); err != nil {
		t.Error(err)
	}
}

// Test that `run` exists
func TestRun(t *testing.T) {
	cmd := newTestRootCmd(t)
	_, _, _, err := runInTempdir(t, cmd, []string{"run", "--help"})
	if err != nil {
		t.Error(err)
	}
}

func TestRunNextflow(t *testing.T) {
	v := newTestViper(t)
	cmd := NewRootCmd(v)
	wd, _, _, err := runInTempdir(t, cmd, []string{"run", "nf-core/raredisease", "--version", "2.4.0"})

	if err == nil {
		t.Error("expected execution to fail but it didn't")
	}

	_, err = findPlumberFile(v)
	if err != nil {
		t.Error(err)
	}

	analysisFilePath := filepath.Join(wd, `.plumber-analysis.json`)
	if _, err := os.Stat(analysisFilePath); err != nil {
		t.Error(err)
	}

	if _, err := os.Stat(analysisFilePath); err != nil {
		t.Error(err)
	}
}

func TestRunSnakemake(t *testing.T) {
	v := newTestViper(t)
	cmd := NewRootCmd(v)
	wd, _, _, err := runInTempdir(t, cmd, []string{"run", "genomic-medicine-sweden/Twist_Solid", "--version", "v0.23.0", "--profile", "slurm_hg19_clinical"})

	if err == nil {
		t.Error("expected execution to fail but it didn't")
	}

	_, err = findPlumberFile(v)
	if err != nil {
		t.Error(err)
	}

	analysisFilePath := filepath.Join(wd, `.plumber-analysis.json`)
	if _, err := os.Stat(analysisFilePath); err != nil {
		t.Error(err)
	}

	if _, err := os.Stat(analysisFilePath); err != nil {
		t.Error(err)
	}

	analysis := plumber.NewAnalysis().WithWorkdir(wd)
	a, err := analysis.Read()
	if err != nil {
		t.Error(err)
	}
	if a.Pipeline.Repo != "genomic-medicine-sweden/Twist_Solid" {
		t.Errorf("expected analysis pipeline %s, got %s", "genomic-medicine-sweden/Twist_Solid", a.Pipeline.Repo)
	}
	if a.Pipeline.Revision != "v0.23.0" {
		t.Errorf("expected analysis pipeline version %s, got %s", "v0.23.0", a.Pipeline.Revision)
	}
	if a.Workdir != wd {
		t.Errorf("expected analysis workdir %s, got %s", wd, a.Workdir)
	}
}

func TestRunWebhooks(t *testing.T) {
	v := newTestViper(t)
	v.Set("webhook-url", "https://example.com")
	v.Set("webhook-api-key", "api-key=thisissecret")

	cmd := NewRootCmd(v)

	_, _, se, err := runInTempdir(t, cmd, []string{"run", "nf-core/raredisease", "--version", "2.4.0"})
	strings.Contains(se, "msg=webhook url=https://example.com")

	if err == nil {
		t.Error("expected command to fail")
	}
}

func TestRunWebhooksInvalidApiKeyFormat(t *testing.T) {
	v := newTestViper(t)
	v.Set("webhook-url", "https://example.com")
	v.Set("webhook-api-key", "thisissecret")

	cmd := NewRootCmd(v)

	_, _, se, err := runInTempdir(t, cmd, []string{"run", "nf-core/raredisease", "--version", "2.4.0"})
	strings.Contains(se, "invalid api key format")

	if err == nil {
		t.Error("expected command to fail")
	}
}
