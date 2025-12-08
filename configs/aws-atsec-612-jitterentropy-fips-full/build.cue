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
]

art: images: aws_atsec_612_jitterentropy_fips_full: {
	metadata: fips: true
	pkgs: list.Concat([packages, aws.fips_612_full])
}
