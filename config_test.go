package plumber

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestConfigCheckout(t *testing.T) {
	d := t.TempDir()

	testcases := []struct {
		name      string
		revision  string
		localPath string
		gitFail   bool
	}{
		{
			name:      "valid revision",
			revision:  "main",
			localPath: d + "/config-files-checkout1",
		},
		{
			name:      "invalid revision",
			revision:  "this-branch-surely-doesnt-exist",
			localPath: d + "/config-files-checkout2",
			gitFail:   true,
		},
		{
			name:      "revision with spaces",
			revision:  "this branch surely doesn't exist",
			localPath: d + "/config-files-checkout2",
			gitFail:   true,
		},
	}

	for _, c := range testcases {
		t.Run(c.name, func(t *testing.T) {
			config := NewConfig("gmc-norr/config-files", c.revision, c.localPath)
			if config.Exists() {
				t.Fatal("local path should not exist")
			}
			if err := config.Clone(); err != nil && !c.gitFail {
				t.Fatal(err)
			} else if err == nil && c.gitFail {
				t.Fatal("git should have failed, but didn't")
			} else if err != nil {
				t.Log(err)
			}

			if c.gitFail {
				return
			}

			if !config.Exists() {
				t.Fatal("local path should exist")
			}
		})
	}

}

func TestNfCoreConfig(t *testing.T) {
	d := t.TempDir()

	testcases := []struct {
		name       string
		pipeline   string
		revision   string
		localPath  string
		shouldFail bool
	}{
		{
			name:      "raredisease",
			pipeline:  "nf-core/raredisease",
			revision:  "main",
			localPath: "config-files-raredisease",
		},
		{
			name:       "no such pipeline",
			pipeline:   "gmc-norr/this-pipeline-doesnt-exist",
			revision:   "main",
			localPath:  "config-files-main",
			shouldFail: true,
		},
	}

	for _, c := range testcases {
		t.Run(c.name, func(t *testing.T) {
			configPath := filepath.Join(d, c.localPath)
			config := NewConfig("gmc-norr/config-files", c.revision, configPath)
			pipeline, err := ParsePipelineName(c.pipeline)
			if err != nil {
				t.Fatal(err)
			}
			nfCoreConfig, err := NewNextflowConfig(pipeline, config)
			if err != nil && !c.shouldFail {
				t.Fatal(err)
			} else if err == nil && c.shouldFail {
				t.Fatal("expected error, got nil")
			}

			if c.shouldFail {
				return
			}

			expectedConfig := filepath.Join(configPath, "nextflow", pipeline.Repo, fmt.Sprintf("%s.config", pipeline.Pipeline))
			if nfCoreConfig.ConfigFile != expectedConfig {
				t.Fatalf("expected config file %s, got %s", expectedConfig, nfCoreConfig.ConfigFile)
			}

			expectedParams := filepath.Join(configPath, "nextflow", pipeline.Repo, "params.yaml")
			if nfCoreConfig.ParamsFile != expectedParams {
				t.Fatalf("expected params file %s, got %s", expectedParams, nfCoreConfig.ParamsFile)
			}
		})
	}
}
