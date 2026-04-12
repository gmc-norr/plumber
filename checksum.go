package plumber

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
)

// ErrChecksumPathNotFound represents an error when a path cannot be found
// when comparing checksums.
type ErrChecksumPathNotFound struct {
	Path string
}

func (err ErrChecksumPathNotFound) Error() string {
	return fmt.Sprintf("path not found: %s", err.Path)
}

// ErrChecksumMismatch represents the error when checksums mismatch for a file.
type ErrChecksumMismatch struct {
	Path     string
	Sum      [16]byte
	OtherSum [16]byte
}

func (err ErrChecksumMismatch) Error() string {
	return fmt.Sprintf("mismatching checksum for %s: %s vs %s", err.Path, err.Sum, err.OtherSum)
}

// FileChecksum represents the md5 checksum of a file.
type FileChecksum struct {
	Path string
	Sum  [16]byte
}

// MarshalText marshals a [FileChecksum] into a newline-terminated string.
// This format is compatible with `md5sum`
func (s FileChecksum) MarshalText() ([]byte, error) {
	text := fmt.Sprintf("%s  %s\n", hex.EncodeToString(s.Sum[:]), s.Path)
	return []byte(text), nil
}

// UnmarshalText unmarshals a file checksum in text format into a [FileChecksum].
func (s *FileChecksum) UnmarshalText(b []byte) error {
	parts := bytes.Split(bytes.TrimSpace(b), []byte{' ', ' '})
	if len(parts) != 2 {
		return fmt.Errorf("invalid format")
	}
	s.Path = string(parts[1])
	if n, err := hex.Decode(s.Sum[:], parts[0]); err != nil {
		return fmt.Errorf("failed to decode checksum: %w", err)
	} else if n != 16 {
		return fmt.Errorf("invalid checksum length: %w", err)
	}
	return nil
}

// Write writes a single [FileChecksum] as a line of text to an [io.Writer].
func (s FileChecksum) Write(w io.Writer) error {
	data, err := s.MarshalText()
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// FileChecksums is a slice of [FileChecksum].
type FileChecksums []FileChecksum

func (s FileChecksums) Write(w io.Writer) error {
	for _, fsum := range s {
		if err := fsum.Write(w); err != nil {
			return err
		}
	}
	return nil
}

// Check will check all checksums in s against the corresponding paths on the file system
// rooted at `root`. The error returned is a join of all errors encountered during the check.
// Unwrap to inspect individual errors. See [Compare].
func (s FileChecksums) Check(root string) error {
	var errs []error
	for _, sum := range s {
		disksum, err := func() ([16]byte, error) {
			path := filepath.Join(root, sum.Path)
			f, err := os.Open(path)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return [16]byte{}, ErrChecksumPathNotFound{Path: path}
				}
				return [16]byte{}, fmt.Errorf("%w: failed to read file: %s", err, sum.Path)
			}
			defer func() {
				_ = f.Close()
			}()
			return checksum(f)
		}()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if disksum != sum.Sum {
			errs = append(errs, fmt.Errorf("mismatching checksum for %s", sum.Path))
		}
	}
	return errors.Join(errs...)
}

// Compare two sets of checksums. If there are checksum mismatches, or a path is missing in
// either set, a non-nil error is returned. If all checksums exist and match, `nil` is returned.
// The returned error is a joint error for all the mismatches or missing files encountered and
// will be of type `ErrChecksumMismatch` or `ErrChecksumPathNotFound`. Unwrap to inspect
// individual errors:
//
//	err := checksums.Compare(other)
//	if err != nil {
//	  errs := err.(interface{ Unwrap() []error }).Unwrap()
//	  // Work with errors here
//	}
//
// Note that the unwrapping will panic if `err` is `nil`.
func (s FileChecksums) Compare(other FileChecksums) error {
	errs := make([]error, 0, 100)
	for _, sum := range s {
		i := slices.IndexFunc(other, func(c FileChecksum) bool {
			return sum.Path == c.Path
		})
		if i == -1 {
			slog.Debug("path not found", "checksum", sum)
			errs = append(errs, ErrChecksumPathNotFound{Path: sum.Path})
			continue
		}
		if sum.Sum != other[i].Sum {
			slog.Debug("sum mismatching", "s", sum.Sum, "other", other[i].Sum)
			errs = append(errs, ErrChecksumMismatch{Path: sum.Path, Sum: sum.Sum, OtherSum: other[i].Sum})
		}
	}
	if len(other) > len(s) {
		slog.Debug("missing files")
		for _, oc := range other {
			i := slices.IndexFunc(s, func(c FileChecksum) bool {
				return oc.Path == c.Path
			})
			if i == -1 {
				errs = append(errs, ErrChecksumPathNotFound{Path: oc.Path})
			}
		}
	}
	return errors.Join(errs...)
}

// ParseChecksums parses checksums in text format into [FileChecksums].
// If the unmarshaling fails, a non-nil error will be returned.
func ParseChecksums(r io.Reader) (FileChecksums, error) {
	s := bufio.NewScanner(r)
	sums := make([]FileChecksum, 0, 100)
	for s.Scan() {
		sum := FileChecksum{}
		if err := sum.UnmarshalText(s.Bytes()); err != nil {
			return nil, err
		}
		sums = append(sums, sum)
	}
	return sums, nil
}

// ReadChecksums parses a file with checksums into [FileChecksums].
// If the unmarshaling fails, a non-nil error will be returned.
func ReadChecksums(path string) (FileChecksums, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	return ParseChecksums(f)
}

// NewFileChecksum creates a new [FileChecksum] for a file at `path` rooted
// at `root`. If the file does not exists, cannot be read, or the checksum
// cannot be computed, a non-nil error is returned.
func NewFileChecksum(root string, path string) (FileChecksum, error) {
	f, err := os.Open(filepath.Join(root, path))
	sum := FileChecksum{
		Path: path,
	}
	if err != nil {
		return sum, err
	}
	defer func() {
		_ = f.Close()
	}()
	s, err := checksum(f)
	if err != nil {
		return sum, err
	}
	sum.Sum = s
	return sum, nil
}

func checksum(r io.Reader) ([16]byte, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return [16]byte{}, err
	}
	return md5.Sum(b), nil
}
