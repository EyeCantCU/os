package aws

import (
	"list"

	"chainguard.dev/wolfi-vm/configs:common"
	"chainguard.dev/wolfi-vm/configs:kernel"
)

// Image sets
// Groups of packages assembled into a functional image flavor.
fips_612_full: list.Concat([
	packages,
	fips_agents,
	common.ssh_key_fetcher,
	common.apk_tools,
	kernel.aws_612_fips,
])

// Package sets
// Groups of packages to assemble an image with.

agents: [
	"amazon-ssm-agent",
	"aws-cli-2",
	"cloud-init",
	"ec2-instance-connect",
	"shadow",
]

fips_agents: [
	"amazon-ssm-agent-fips",
	"aws-cli-2",
	"cloud-init",
	"ec2-instance-connect",
	"shadow",
]

packages: [
	"amazon-ec2-net-utils",
	"amazon-ec2-utils",
	"aws-configs",
	"busybox",
	"chrony",
	"chrony-aws",
	"ec2-user",
]
