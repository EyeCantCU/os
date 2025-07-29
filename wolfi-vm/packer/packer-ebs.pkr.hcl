packer {
  required_plugins {
    amazon = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

variable "source_ami" {
  type    = string
  default = null
  description = "The source AMI ID to customize."
}

variable "region" {
  type    = string
  default = "us-east-1"
  description = "The AWS region to build the AMI in"
}

variable "packages" {
  type    = list(string)
  default = ["grype"]
  description = "List of packages to install via apk"
}

// Use profile if you have multiple AWS profiles in credentials file
variable "aws_profile" {
  type        = string
  default     = "default"
  description = "AWS profile to use for credentials"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "amazon-ebs" "chainguard" {
  communicator  = "ssh"
  ssh_file_transfer_method = "sftp" # scp
  ssh_username  = "root"

  // Add credentials if provided as variables
  ami_name      = "chainguard-custom-${local.timestamp}"
  instance_type = "t3.micro"
  profile       = var.aws_profile
  region        = var.region
  source_ami    = var.source_ami

  run_tags = {
    Name        = "packer-temporary-instance-${local.timestamp}"
    CreatedBy   = "Packer"
    Project     = "AMI Creation"
    Environment = "Development"
    Temporary   = "Yes"
  }

  run_volume_tags = {
    Name        = "packer-debug-volume-${local.timestamp}"
    CreatedBy   = "Packer"
  }

  // Show VPC information
  vpc_filter {
    filters = {
      isDefault = "true"
    }
  }

  // Optional: Add tags to your AMI
  tags = {
    Name        = "chainguard Custom"
    Environment = "Development"
    Builder     = "Packer"
  }
}

build {
  sources = ["source.amazon-ebs.chainguard"]

  provisioner "shell" {
    inline = [
      "echo 'Installing packages: ${join(" ", var.packages)}'",
      "sudo apk update",
      "sudo apk add ${join(" ", var.packages)}"
    ]
  }

  provisioner "shell" {
    inline = [
      "echo 'running grype scan'",
      "sudo grype /"
    ]
  }

  // Optionally add more customizations
  post-processor "manifest" {
    output = "manifest.json"
    strip_path = true
  }
}
