package plumber

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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

func (r GitRepo) Sync(ctx context.Context) error {
	if r.LocalPath == "" {
		r.LocalPath = filepath.Base(r.Url.Path)
	}
	if info, err := os.Stat(r.LocalPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return r.Clone(ctx)
		}
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("local path is not a directory")
	}

	if err := r.Fetch(ctx); err != nil {
		return err
	}

	return nil
}

func (r GitRepo) Fetch(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "fetch")
	cmd.Dir = r.LocalPath
	slog.Debug("running git command", "cmd", cmd.String(), "workdir", cmd.Dir)
	return cmd.Run()
}

func (r GitRepo) Clone(ctx context.Context) error {
	argv := []string{
		"clone",
		r.Url.String(),
	}
	if r.LocalPath != "" {
		argv = append(argv, r.LocalPath)
	}
	cmd := exec.CommandContext(ctx, "git", argv...)
	slog.Debug("running git command", "cmd", cmd.String(), "workdir", cmd.Dir)
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, o)
	}
	return nil
}

func (r GitRepo) Checkout(ctx context.Context, version string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", version, "--")
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
