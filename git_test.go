package plumber

import (
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
