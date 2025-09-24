package pyenv

import (
	"testing"
)

func TestVersionFromString(t *testing.T) {
	testcases := []struct {
		version string
		expect  PythonVersion
	}{
		{
			version: "3",
			expect:  PythonVersion([]string{"3"}),
		},
		{
			version: "3.10",
			expect:  PythonVersion([]string{"3", "10"}),
		},
		{
			version: "3.10.7",
			expect:  PythonVersion([]string{"3", "10", "7"}),
		},
	}

	for _, c := range testcases {
		t.Run(c.version, func(t *testing.T) {
			v := VersionFromString(c.version)
			if len(v) != len(c.expect) {
				t.Errorf("expected %d components, got %d", len(c.expect), len(v))
			}
			for i := range len(v) {
				if v[i] != c.expect[i] {
					t.Errorf("expected component %d to be %s, got %s", i+1, c.expect[i], v[i])
				}
			}
		})
	}
}

func TestIsVersion(t *testing.T) {
	testcases := []struct {
		name  string
		v1    PythonVersion
		v2    PythonVersion
		match bool
	}{
		{
			name:  "full vs major",
			v1:    PythonVersion{"3", "10", "7"},
			v2:    PythonVersion{"3"},
			match: true,
		},
		{
			name:  "full vs minor",
			v1:    PythonVersion{"3", "10", "7"},
			v2:    PythonVersion{"3", "10"},
			match: true,
		},
		{
			name:  "full vs full",
			v1:    PythonVersion{"3", "10", "7"},
			v2:    PythonVersion{"3", "10", "7"},
			match: true,
		},
		{
			name:  "minor vs full",
			v1:    PythonVersion{"3", "10"},
			v2:    PythonVersion{"3", "10", "7"},
			match: false,
		},
		{
			name:  "empty vs full",
			v1:    PythonVersion{},
			v2:    PythonVersion{"3", "10", "7"},
			match: false,
		},
	}

	for _, c := range testcases {
		t.Run(c.name, func(t *testing.T) {
			if c.match != c.v1.Is(c.v2) {
				t.Errorf("expected match to be %t, got %t", c.match, c.v1.Is(c.v2))
			}
		})
	}
}
