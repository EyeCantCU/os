# Packer Multi-Cloud Configuration Guide

This repository contains Packer configurations for building custom machine images across AWS, GCP, and Azure cloud providers.
Each configuration installs and runs the [Grype](https://github.com/anchore/grype) vulnerability scanner.

## Prerequisites

- [Packer](https://www.packer.io/downloads) (version 1.7.0+) installed
- Cloud provider CLI tools:
  - [AWS CLI](https://aws.amazon.com/cli/)
  - [Google Cloud SDK](https://cloud.google.com/sdk/docs/install)
  - [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)
- Valid credentials for the cloud providers you plan to use

## Configuration Files

This repository includes three Packer configuration files:

1. `packer-ebs.pkr.hcl` - AWS configuration
2. `packer-gcp.pkr.hcl` - GCP configuration
3. `packer-azure.pkr.hcl` - Azure configuration

## Getting Started

### 1. Initialize Packer

Before using any of the configuration files, initialize Packer to download the required plugins:

```bash
# For AWS
packer init packer-ebs.pkr.hcl

# For GCP
packer init packer-gcp.pkr.hcl

# For Azure
packer init packer-azure.pkr.hcl
```

### 2. Authenticate with Cloud Providers

#### AWS Authentication

```bash
# Configure AWS credentials
aws configure sso --profile cg-dev
```

#### GCP Authentication

```bash
# Login with gcloud
gcloud auth login

# Set the active project
gcloud config set project wolfi-vm
```

#### Azure Authentication

```bash
# Login with az CLI
az login
az account set --subscription "ad60e736-b0ce-432e-b77c-4b8218f452ae"
```

### 3. Build Images

#### AWS

```bash
packer build \
  -var="region=us-east-1" \
  -var="aws_profile=cg-dev" \
  -var="source_ami=ami-0967f45699263f38a" \
  packer-ebs.pkr.hcl
```

Required variables:
- `region` (optional, defaults to `us-east-1`)
- `aws_profile`
- `source_ami`

#### GCP

```bash
packer build \
  -var="project_id=wolfi-vm" \
  -var="source_image=chainguard-google-agents-amd64-packer" \
  packer-gcp.pkr.hcl
```

Required variables:
- `project_id` (defaults to `wolfi-vm`)
- `source_image` (required)
- `zone` (optional, defaults to `us-central1-a`)

#### Azure

```bash
packer build \
  -var="subscription_id=ad60e736-b0ce-432e-b77c-4b8218f452ae" \
  -var="resource_group_name=chainguard-vms" \
  -var="source_image_offer=..." \
  packer-azure.pkr.hcl
```

Required variables:
- `subscription_id` (required)
- `resource_group_name` (required)
- `location` (optional, defaults to `eastus`)

## Customization

### Custom Packages

All configurations support a `packages` variable that allows you to specify additional packages to install.

Example:
```bash
packer build \
  -var='packages=["grype", "vim", "curl", "jq"]' \
  packer-ebs.pkr.hcl
```

### Debugging

To enable verbose logging, add the `-debug` flag:

```bash
packer build -debug packer-ebs.pkr.hcl
```

## TODO:

- Azure needs a new image
- Azure needs a more detailed account setup
- Azure needs tests, this configuration is **untested**
