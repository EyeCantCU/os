//go:build withauth

/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"context"
	"os"
	"testing"
	"time"

	"chainguard.dev/apko/pkg/apk/auth"
	"chainguard.dev/apko/pkg/build"
	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apko/pkg/tarfs"
	"github.com/anchore/syft/syft/file"
	"github.com/anchore/syft/syft/format/syftjson"
	"github.com/anchore/syft/syft/pkg"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

var ic types.ImageConfiguration = types.ImageConfiguration{
	Archs: []types.Architecture{
		"amd64",
	},
	Contents: types.ImageContents{
		Repositories: []string{
			"https://packages.wolfi.dev/os",
			"https://packages.cgr.dev/extras",
			"https://apk.cgr.dev/chainguard-private",
		},
		Keyring: []string{
			"https://packages.wolfi.dev/os/wolfi-signing.rsa.pub",
			"https://packages.cgr.dev/extras/chainguard-extras.rsa.pub",
		},
		Packages: []string{
			"ca-certificates-bundle=20241121-r1",
			"chainguard-baselayout=20230214-r12",
			"curl=8.12.1-r0",
			"cyrus-sasl=2.1.28-r7",
			"gdbm=1.24-r3",
			"glibc-locale-posix=2.40-r23",
			"glibc=2.40-r23",
			"heimdal-libs=7.8.0-r9",
			"keyutils-libs=1.6.3-r31",
			"krb5-conf=1.0-r4",
			"krb5-libs=1.21.3-r2",
			"ld-linux=2.40-r23",
			"libbrotlicommon1=1.1.0-r7",
			"libbrotlidec1=1.1.0-r7",
			"libcom_err=1.47.2-r21",
			"libcrypt1=2.40-r23",
			"libcrypto3=3.4.1-r1",
			"libcurl-openssl4=8.12.1-r0",
			"libevent=2.1.12-r8",
			"libgcc=14.2.0-r10",
			"libidn2=2.3.8-r0",
			"libldap=2.6.9-r0",
			"libnghttp2-14=1.65.0-r0",
			"libpsl=0.21.5-r6",
			"libssl3=3.4.1-r1",
			"libunistring=1.4.1-r1",
			"libverto=0.3.2-r6",
			"libxcrypt=4.4.38-r1",
			"linux-boot-configuration=6.13.5-r3",
			"linux-boot-installed=6.13.5-r3",
			"mattmoor-chainit-init=0.0.9-r20",
			"mattmoor-chainit=0.0.9-r20",
			"ncurses-terminfo-base=6.5_p20241228-r1",
			"ncurses=6.5_p20241228-r1",
			"readline=8.3-r1",
			"sqlite-libs=3.49.1-r1",
			"systemd-boot-installed=257.4-r0",
			"wolfi-baselayout=20230201-r18",
			"zlib=1.3.1-r6",
		},
	},
	Entrypoint: types.ImageEntrypoint{
		Type:     "",
		Command:  "/usr/bin/curl",
		Services: nil,
	},
	Cmd: "--fail file:///etc/apko.json",
}

func buildImage(t *testing.T) v1.Layer {
	// We should comfortably be able to convert all of these images
	// in under a minute.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	t.Cleanup(cancel)

	arch := types.ParseArchitecture("amd64")
	bc, err := build.New(ctx, tarfs.New(),
		build.WithAuthenticator(auth.CGRAuth{}),
		build.WithArch(arch),
		build.WithImageConfiguration(ic),
	)
	if err != nil {
		t.Fatalf("build.New() failed with %v", err)
	}

	_, layer, err := bc.BuildLayer(ctx)
	if err != nil {
		t.Fatalf("bc.BuildLayer() failed with %v", err)
	}

	return layer
}

func TestCreateSyftSBOM(t *testing.T) {
	layer := buildImage(t)

	sbomdata, err := CreateSyftSBOMFromLayer(context.Background(), layer)
	if err != nil {
		t.Fatalf("CreateSyftSBOM() failed with %v", err)
	}

	testdata, err := os.Open("testdata/syft.sbom.json")
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer testdata.Close()

	decoder := syftjson.NewFormatDecoder()
	expect, _, _, err := decoder.Decode(testdata)
	if err != nil {
		t.Fatalf("failed to decode SBOM: %v", err)
	}
	sbom, _, _, err := decoder.Decode(sbomdata)
	if err != nil {
		t.Fatalf("failed to decode SBOM: %v", err)
	}

	if !cmp.Equal(expect.Artifacts.Packages.Sorted(), sbom.Artifacts.Packages.Sorted(),
		cmpopts.IgnoreUnexported(pkg.Package{}),
		cmpopts.IgnoreUnexported(pkg.LicenseSet{}),
		cmpopts.IgnoreUnexported(file.LocationSet{})) {
		// visually show what is different
		diff := cmp.Diff(expect.Artifacts.Packages.Sorted(), sbom.Artifacts.Packages.Sorted(),
			cmpopts.IgnoreUnexported(pkg.Package{}),
			cmpopts.IgnoreUnexported(pkg.LicenseSet{}),
			cmpopts.IgnoreUnexported(file.LocationSet{}),
		)
		t.Fatalf("Expected syft SBOM is different from expected(-want +got):\n%s", diff)
	}
}
