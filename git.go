package plumber

import (
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
)

type GitRepo struct {
	Url       *url.URL
	LocalPath string
}

func NewGitRepo(path string) (GitRepo, error) {
	r := GitRepo{}

	u, err := url.Parse(path)
	if err != nil {
		return r, err
	}

	r.Url = u
	return r, nil
}

func (r GitRepo) Clone() error {
	argv := []string{
		"clone",
		r.Url.RawPath,
	}
	if r.LocalPath != "" {
		argv = append(argv, r.LocalPath)
	}
	cmd := exec.Command("git", argv...)
	slog.Debug("running git command", "cmd", cmd.String(), "workdir", cmd.Dir)
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, o)
	}
	return nil
}

func (r GitRepo) Checkout(version string) error {
	cmd := exec.Command("git", "checkout", version)
	cmd.Dir = r.LocalPath
	slog.Debug("running git command", "cmd", cmd.String(), "workdir", cmd.Dir)
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, o)
	}
	return nil
}

func (r GitRepo) IsLocal() bool {
	return r.Url.Hostname() == "" && r.Url.Scheme == ""
}
