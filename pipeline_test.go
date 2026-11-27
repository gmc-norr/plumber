package plumber

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestRunWithLargeOutput checks that run returns if it is supposed to.
// This could potentially be flaky since it relies on timing, but I've
// tried to get around this by writing a marker file in the subprocess
// and wait for this to exist before waiting for the output.
func TestRunWithLargeOutput(t *testing.T) {
	cases := []struct {
		name        string
		outputSize  int
		shouldError bool
		file        string
	}{
		{
			name:        "stdout too large",
			outputSize:  128 * 1024,
			shouldError: true,
			file:        "stdout",
		},
		{
			name:        "stdout just right",
			outputSize:  4 * 1024,
			shouldError: false,
			file:        "stdout",
		},
		{
			name:        "stderr too large",
			outputSize:  128 * 1024,
			shouldError: true,
			file:        "stderr",
		},
		{
			name:        "stderr just right",
			outputSize:  4 * 1024,
			shouldError: false,
			file:        "stderr",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpdir := t.TempDir()
			startedMarker := filepath.Join(tmpdir, "test_started")
			// Have the command touch a marker file to indicate that it started, and then output data that will be consumed
			script := fmt.Sprintf("touch %q && head -c %d /dev/zero", startedMarker, c.outputSize)
			if c.file == "stderr" {
				script += " >&2"
			}
			cmd := exec.Command("sh", "-c", script)

			done := make(chan struct{})
			go func() {
				err := run(cmd, 10, 0)
				if err != nil != c.shouldError {
					t.Errorf("should error: %t, error: %v", c.shouldError, err)
				} else if err != nil && c.shouldError {
					if !strings.Contains(err.Error(), c.file) {
						t.Errorf("error should contain %s: %v", c.file, err)
					}
				}
				t.Logf("run error: %v", err)
				close(done)
			}()

			completionTimeout := 2 * time.Second

			// Use the deadline of the whole test suite for seeing the marker file
			var deadline <-chan time.Time
			if d, ok := t.Deadline(); ok {
				deadline = time.After(time.Until(d) - completionTimeout)
			}
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()

			for {
				if _, err := os.Stat(startedMarker); err == nil {
					break
				} else if !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("stat marker file: %v", err)
				}
				select {
				case <-ticker.C:
				case <-deadline:
					t.Fatal("execution never started before deadline")
				}
			}

			select {
			case <-done:
				return
			case <-time.After(completionTimeout):
				t.Errorf("function didn't return in %s after execution started, likely deadlock", completionTimeout)
			}
		})
	}
}
