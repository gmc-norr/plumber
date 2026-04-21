package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

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
	cmd := newTestRootCmd(t)
	wd, so, se, err := runInTempdir(t, cmd, []string{"nextflow", "run", "nf-core/raredisease", "--version", "2.4.0", "-l", "debug"})
	t.Log(cmd.Args)
	analysisFilePath := filepath.Join(wd, `.plumber-analysis.json`)
	t.Logf("stdout: %s", so)
	t.Logf("stderr: %s", se)
	if _, err := os.Stat(analysisFilePath); err != nil {
		t.Error(err)
	}
	if err == nil {
		t.Error("expected execution to fail but it didn't")
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

// Test that nextflow run outputs `.plumber-analysis.json`
func TestHydraRunAnalyisFile(t *testing.T) {
	cmd := newTestRootCmd(t)
	wd, so, se, err := runInTempdir(t, cmd, []string{"hydra", "run", "genomic-medicine-sweden/Twist_Solid", "--version", "v0.23.0", "-l", "debug", "--profile", "slurm_hg19_research"})
	analysisFilePath := filepath.Join(wd, `.plumber-analysis.json`)
	if _, err := os.Stat(analysisFilePath); err != nil {
		t.Error(err)
		t.Logf("stdout: %s", so)
		t.Logf("stderr: %s", se)
	}
	if err == nil {
		t.Error("expected execution to fail but it didn't")
	}
}
