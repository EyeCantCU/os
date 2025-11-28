package main

import (
	"list"
	"chainguard.dev/wolfi-vm/configs:aws"
)

packages: [
	"bison",
	"build-base",
	"diffutils",
	"elfutils",
	"elfutils-dev",
	"findutils",
	"flex",
	"gawk",
	"gmp",
	"gmp-dev",
	"gnutar",
	"mpc",
	"mpc-dev",
	"mpfr",
	"mpfr-dev",
	"openssl",
	"openssl-dev",
	"pahole",
	"pahole-dev",
	"perl",
	"python3",
	"qemu",
	"wolfi-base",
	"zstd",
	"linux-jitterentropy-6.12=6.12.54-r3",
	"linux-jitterentropy-6.12-test-objects=6.12.54-r3",
]

art: images: aws_atsec_jitterentropy_fips_full: {
	metadata: fips: true
	pkgs: list.Concat([packages, aws.fips_full])
}
