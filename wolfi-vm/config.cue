package main

import (
	"list"
	artv1 "chainguard.dev/wolfi-vm/cue/cfg/art/v1"
	"chainguard.dev/wolfi-vm/cue/cfg/apko"
)

// Configuration for wolfi-vm. It defines the settings that a user can
// change, instead of exposing the entire art.#Config object.
#Config: {
	runtime_repositories: [...string] | *[]
	build_repositories: [...string] | *[]
	repositories: [...string] | *[]
	packages: [...string] | *[]
	devPackages: [...string] | *[]
	archs: [...apko.#Architecture]
}

cfg: #Config

// Concated-elsehwere, see image.cue
nonFipsDefaultPkgs: [ "grype" ]
fipsDefaultPkgs: [ "grype-fips" ]

// Combine repo config with default values to create config struct that is read
// by the art CLI.
artConfig: artv1.#Config & {
	contents: {
		runtime_repositories: list.Concat([cfg.runtime_repositories, [
			"https://virtualapk.cgr.dev/0ac7ff905850c35723a7f376e10d007c958c45c8/chainguard",
			"https://virtualapk.cgr.dev/0ac7ff905850c35723a7f376e10d007c958c45c8/extra-packages",
		]])
		build_repositories: list.Concat([cfg.build_repositories, [
			"https://apk.cgr.dev/chainguard",
			"https://apk.cgr.dev/extra-packages",
			"https://apk.cgr.dev/chainguard-private",
		]])
		packages: list.Concat([cfg.packages, [
			"ca-certificates",
			"chainguard-baselayout",
			"coreutils",
			"busybox",
			"dbus",
			"iproute2",
			"kmod",
			"kmod-libs",
			"linux-pam",
			"libpwquality",
			"mount",
			"openssh-server",
			"openssh-service",
			"openssh-sftp-server",
			"polkit",
			"sudo-rs",
			"systemd-boot",
			"systemd-boot-installed",
			"systemd-default-network",
			"systemd-init",
			"systemd-logind-service",
			"udev",
			"umount",
			"util-linux-misc",
			"xfsprogs",
		]])
		devPackages: list.Concat([cfg.devPackages, []])
	}

	archs: [
		if len(cfg.archs) > 0 {cfg.archs},
		["amd64", "arm64"],
	][0]

	ociRepository:       ""
	ociCustomRepository: ""
}
