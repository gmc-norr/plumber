package plumber

import (
	"bytes"
	"os/exec"
	"testing"
)

func TestIsLocal(t *testing.T) {
	testcases := []struct {
		Name    string
		Url     string
		IsLocal bool
	}{
		{
			Name:    "github repo",
			Url:     "https://github.com/gmc-norr/config-files",
			IsLocal: false,
		},
		{
			Name:    "absolute path",
			Url:     "/tmp/config-files",
			IsLocal: true,
		},
		{
			Name:    "github repo",
			Url:     "config-files",
			IsLocal: true,
		},
	}
	for _, c := range testcases {
		t.Run(c.Name, func(t *testing.T) {
			r, _ := NewGitRepo(c.Url)
			if r.IsLocal() != c.IsLocal {
				t.Errorf("IsLocal should be %t", c.IsLocal)
			}
		})
	}
}

func gitReset(t *testing.T, path string, args ...string) error {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"reset"}, args...)...)
	cmd.Dir = path
	return cmd.Run()
}

func headHash(t *testing.T, path string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "rev-parse", "HEAD")
	cmd.Dir = path
	o, err := cmd.Output()
	return string(bytes.TrimSpace(o)), err
}

func TestSync(t *testing.T) {
	testcases := []struct {
		name   string
		url    string
		rev    string
		commit string
	}{
		{
			name: "fast-forward possible",
			url:  "https://github.com/gmc-norr/config-files",
			rev:  "main",
		},
		{
			name:   "checked out tag",
			url:    "https://github.com/gmc-norr/config-files",
			rev:    "v0.4.0",
			commit: "0a33360174302b2e9247553f0086c93697e67ab4",
		},
		{
			name:   "checked out commit",
			url:    "https://github.com/gmc-norr/config-files",
			rev:    "0a33360174302b2e9247553f0086c93697e67ab4",
			commit: "0a33360174302b2e9247553f0086c93697e67ab4",
		},
	}
	for _, c := range testcases {
		t.Run(c.name, func(t *testing.T) {
			r, _ := NewGitRepo(c.url)
			r.LocalPath = t.TempDir()
			if err := r.Clone(t.Context()); err != nil {
				t.Fatal(err)
			}
			// The hash of the origin head
			if c.commit == "" {
				hash, err := headHash(t, r.LocalPath)
				if err != nil {
					t.Fatal(err)
				}
				c.commit = hash
			}
			if err := gitReset(t, r.LocalPath, "--hard", "HEAD~3"); err != nil {
				t.Fatal(err)
			}
			if err := r.Sync(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := r.Checkout(t.Context(), c.rev); err != nil {
				t.Fatal(err)
			}
			hash, err := headHash(t, r.LocalPath)
			if err != nil {
				t.Fatal(err)
			}
			if c.commit != hash {
				t.Errorf("expected HEAD to be %s, got %s", hash, hash)
			}
		})
	}
}
