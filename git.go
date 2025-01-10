package plumber

import (
	"fmt"
	"log/slog"
	"os/exec"
)

type GitRepo struct {
	Url       string
	LocalPath string
}

func NewGitRepo(url string) GitRepo {
	return GitRepo{
		Url: url,
	}
}

func (r GitRepo) Clone() error {
	argv := []string{
		"clone",
		r.Url,
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
