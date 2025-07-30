/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"chainguard.dev/apko/pkg/apk/auth"
	"chainguard.dev/apko/pkg/apk/expandapk"
)

// for the future, to be set with ldflags to freeze versions
var (
	qemuSystemVersion = ""
	kernelVersion     = ""
)

func fetchPackageVersion(repo, pkg, apkArch string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/%s/APKINDEX.tar.gz", repo, apkArch), nil)
	if err != nil {
		return "", fmt.Errorf("http.NewRequest() failed with %w", err)
	}
	auth := auth.CGRAuth{}
	err = auth.AddAuth(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("auth.AddAuth() failed with %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http.DefaultClient.Do() failed with %w", err)
	}
	defer res.Body.Close()

	apkindex, err := gzip.NewReader(res.Body)
	if err != nil {
		return "", fmt.Errorf("gzip.NewReader() failed with: %w", err)
	}
	defer apkindex.Close()

	scanner := bufio.NewScanner(apkindex)
	matchPackage := regexp.MustCompile(`^P:` + pkg + "$")
	getNext := false
	version := ""

	for scanner.Scan() {
		line := scanner.Text()
		if matchPackage.MatchString(line) {
			getNext = true
		}
		if !strings.HasPrefix(line, "P:"+pkg) && getNext {
			version = strings.Split(line, ":")[1]
			getNext = false
		}
	}

	return version, nil
}

func fetchAndUnpack(repo, pkg, apkArch, version, directory string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/%s/%s-%s.apk", repo, apkArch, pkg, version), nil)
	if err != nil {
		return fmt.Errorf("http.NewRequest() failed with %w", err)
	}
	auth := auth.CGRAuth{}
	err = auth.AddAuth(context.Background(), req)
	if err != nil {
		return fmt.Errorf("auth.AddAuth() failed with %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http.DefaultClient.Do() failed with %w", err)
	}
	defer res.Body.Close()

	rds, err := expandapk.Split(res.Body)
	if err != nil {
		return fmt.Errorf("expandapk.Split() failed with %w", err)
	}
	data := rds[len(rds)-1] // The last stream is always the data.

	gz, err := gzip.NewReader(data)
	if err != nil {
		return fmt.Errorf("gzip.NewReader() failed with %w", err)
	}

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar.Reader.Next() failed with %w", err)
		}

		// nolint:gosec // We trust the caller to provide a valid path.
		target := filepath.Join(directory, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("os.MkdirAll() failed with %w", err)
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("os.OpenFile() failed with %w", err)
			}
			// nolint:gosec // We trust we're downloading our own packages.
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("io.Copy() failed with %w", err)
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("os.Symlink() failed with %w", err)
			}

		// We don't have other types in these packages currently.
		default:
			return fmt.Errorf("unsupported tar header type %v", header.Typeflag)
		}
	}

	return nil
}

func FetchKernel(destdir, apkArch string) (string, error) {
	td, err := os.MkdirTemp(destdir, "kernel")
	if err != nil {
		return "", fmt.Errorf("os.MkdirTemp() failed with %w", err)
	}

	if kernelVersion == "" {
		kernelVersion, err = fetchPackageVersion("apk.cgr.dev/chainguard-private", "linux", apkArch)
		if err != nil {
			return "", fmt.Errorf("failed to fetch package version: %w", err)
		}
	}

	err = fetchAndUnpack("apk.cgr.dev/chainguard-private", "linux", apkArch, kernelVersion, td)
	if err != nil {
		return "", fmt.Errorf("fatch failed with %w", err)
	}

	kernel, err := os.Readlink(filepath.Join(td, "boot/vmlinuz"))
	if err != nil {
		return "", fmt.Errorf("fatch failed with %w", err)
	}

	return filepath.Join(td, "boot", kernel), nil
}

func FetchBios(destdir, apkArch string) (string, error) {
	td, err := os.MkdirTemp(destdir, "bios")
	if err != nil {
		return "", fmt.Errorf("os.MkdirTemp() failed with %w", err)
	}

	if qemuSystemVersion == "" {
		qemuSystemVersion, err = fetchPackageVersion("apk.cgr.dev/chainguard", fmt.Sprintf("qemu-system-%s", apkArch), apkArch)
		if err != nil {
			return "", fmt.Errorf("failed to fetch package version: %w", err)
		}
	}

	err = fetchAndUnpack("apk.cgr.dev/chainguard", fmt.Sprintf("qemu-system-%s", apkArch), apkArch, qemuSystemVersion, td)
	if err != nil {
		return "", fmt.Errorf("fatch failed with %w", err)
	}

	return filepath.Join(td, fmt.Sprintf("usr/share/qemu/edk2-%s-code.fd", apkArch)), nil
}
