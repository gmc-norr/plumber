package pyenv

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

var ErrPythonVersionNotFound error = errors.New("python version not found")

type PythonVersion []string

func (v PythonVersion) String() string {
	return strings.Join(v, ".")
}

func VersionFromString(v string) PythonVersion {
	components := strings.Split(v, ".")
	return PythonVersion(components)
}

// Is compares two Python versons and returns true if v is equal to other and
// other is not more specific than v. Python version 3.10.7 is 3.10, but version
// 3.10 is not 3.10.7.
func (v PythonVersion) Is(other PythonVersion) bool {
	if len(other) > len(v) {
		return false
	}
	for i := range len(other) {
		if v[i] != other[i] {
			return false
		}
	}
	return true
}

type Environment struct {
	Version PythonVersion
	Name    string
	Created bool
}

func (e *Environment) Create() error {
	if e.Created {
		return nil
	}
	if _, err := Version(); err != nil {
		return fmt.Errorf("unable to locate pyenv: %w", err)
	}
	if err := HasPython(e.Version); errors.Is(err, ErrPythonVersionNotFound) {
		if err := Install(e.Version); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	cmd := exec.Command("pyenv", "virtualenv", e.Version.String(), e.Name)
	res, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to create virtual environment: %s, %w", string(res), err)
	}
	e.Created = true
	return nil
}

func (e *Environment) Exists() (bool, error) {
	cmd := exec.Command("pyenv", "versions", "--bare")
	res, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to list versions: %w", err)
	}
	lines := strings.SplitSeq(string(res), "\n")
	for v := range lines {
		if v == e.Name {
			version, err := e.PythonVersion()
			if err != nil {
				return false, err
			}
			slog.Debug("found matching virtualenv, checking python version", "requested_version", e.Version.String(), "existing_version", version.String())
			if version.Is(e.Version) {
				e.Created = true
				return true, nil
			} else {
				return true, fmt.Errorf("environment exists, but python versions mismatch: requested %s, found %s", e.Version.String(), version.String())
			}
		}
	}
	return false, nil
}

func (e Environment) PythonVersion() (PythonVersion, error) {
	var v PythonVersion
	cmd := exec.Command("python", "--version")
	cmd.Env = append(cmd.Env, "PYENV_VERSION="+e.Name)
	res, err := cmd.Output()
	if err != nil {
		return v, fmt.Errorf("failed to get python version: %w", fmt.Errorf("%s: %w", string(res), err))
	}
	version := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(string(res)), "python"))
	return VersionFromString(version), nil
}

func Version() (string, error) {
	cmd := exec.Command("pyenv", "--version")
	res, err := cmd.Output()
	return strings.TrimSpace(string(res)), err
}

func Install(version PythonVersion) error {
	cmd := exec.Command("pyenv", "install", "-s", version.String())
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func HasPython(version PythonVersion) error {
	cmd := exec.Command("pyenv", "versions", "--bare", "--skip-aliases", "--skip-envs")

	res, err := cmd.Output()
	if err != nil {
		slog.Error("pyenv command failed", "error", err)
		return err
	}

	versions := strings.SplitSeq(string(res), "\n")
	for v := range versions {
		pv := VersionFromString(v)
		if pv.Is(version) {
			return nil
		}
	}

	return fmt.Errorf("version %s: %w", version, ErrPythonVersionNotFound)
}
