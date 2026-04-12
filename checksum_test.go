package plumber

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"log/slog"
	"os"
	"slices"
	"testing"
)

func TestFileChecksumMarshal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	testcases := []struct {
		name     string
		checksum FileChecksum
		expect   string
		error    bool
	}{
		{
			name: "relative path checksum",
			checksum: FileChecksum{
				Path: "./this/file/is/good.txt",
			},
		},
		{
			name: "absolute path checksum",
			checksum: FileChecksum{
				Path: "/this/file/is/good.txt",
			},
		},
	}

	for _, c := range testcases {
		t.Run(c.name, func(t *testing.T) {
			c.checksum.Sum = md5.Sum([]byte(c.checksum.Path))
			var b []byte
			buf := bytes.NewBuffer(b)
			if err := c.checksum.Write(buf); err != nil {
				if !c.error {
					t.Fatalf("got error: %s", err)
				}
			} else if c.error {
				t.Fatalf("got no error, expected one")
			}
			parts := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte{' ', ' '})
			if len(parts) != 2 {
				t.Fatal("invalid format of checksum string")
			}
			if !slices.Equal(parts[1], []byte(c.checksum.Path)) {
				t.Errorf("expected second part to be path %q", c.checksum.Path)
			}
			hash := hex.EncodeToString(c.checksum.Sum[:])
			if string(parts[0]) != hash {
				t.Errorf("expected first part to be hash %q", hash)
			}

			var unmarshaled FileChecksum
			if err := unmarshaled.UnmarshalText(buf.Bytes()); err != nil {
				t.Errorf("failed to unmarshal text checksum: %s", err)
			}
			if unmarshaled != c.checksum {
				t.Error("unmarshaled version not the same as the original")
			}
		})
	}
}

func TestFileChecksumCompare(t *testing.T) {
	testcases := []struct {
		name       string
		sums       FileChecksums
		other      FileChecksums
		error      bool
		errorCount int
	}{
		{
			name: "matching",
			sums: FileChecksums{
				FileChecksum{
					Path: "path/to/file1.txt",
				},
				FileChecksum{
					Path: "path/to/file2.txt",
				},
			},
			other: FileChecksums{
				FileChecksum{
					Path: "path/to/file1.txt",
				},
				FileChecksum{
					Path: "path/to/file2.txt",
				},
			},
		},
		{
			name:       "mismatching extra files",
			error:      true,
			errorCount: 1,
			sums: FileChecksums{
				FileChecksum{
					Path: "path/to/file1.txt",
				},
			},
			other: FileChecksums{
				FileChecksum{
					Path: "path/to/file1.txt",
				},
				FileChecksum{
					Path: "path/to/file2.txt",
				},
			},
		},
		{
			name:       "mismatching missing files",
			error:      true,
			errorCount: 1,
			sums: FileChecksums{
				FileChecksum{
					Path: "path/to/file1.txt",
				},
				FileChecksum{
					Path: "path/to/file2.txt",
				},
			},
			other: FileChecksums{
				FileChecksum{
					Path: "path/to/file1.txt",
				},
			},
		},
		{
			name:       "multiple mismatching missing files",
			error:      true,
			errorCount: 2,
			sums: FileChecksums{
				FileChecksum{
					Path: "path/to/file1.txt",
				},
				FileChecksum{
					Path: "path/to/file2.txt",
				},
				FileChecksum{
					Path: "path/to/file3.txt",
				},
			},
			other: FileChecksums{
				FileChecksum{
					Path: "path/to/file1.txt",
				},
			},
		},
	}

	for _, c := range testcases {
		for _, sum := range c.sums {
			sum.Sum = md5.Sum([]byte(sum.Path))
		}
		for _, sum := range c.other {
			sum.Sum = md5.Sum([]byte(sum.Path))
		}
		t.Run(c.name, func(t *testing.T) {
			err := c.sums.Compare(c.other)
			if err != nil && !c.error {
				t.Errorf("did not expect an error, got %v", err)
			}
			if err == nil && c.error {
				t.Errorf("expected error, got nil")
			}
			if err != nil {
				t.Logf("error: %v", err)
				errs := err.(interface{ Unwrap() []error }).Unwrap()
				if len(errs) != c.errorCount {
					t.Errorf("expected %d errors, got %d", c.errorCount, len(errs))
				}
			}
		})
	}
}
