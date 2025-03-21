packer {
  required_plugins {
    googlecompute = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/googlecompute"
    }
  }
}

variable "source_image" {
  type    = string
  default = null
  description = "The source image to customize"
}

variable "project_id" {
  type    = string
  default = "wolfi-vm"
  description = "The GCP project ID to build the image in"
}

variable "zone" {
  type    = string
  default = "us-central1-a"
  description = "The GCP zone to build the image in"
}

variable "packages" {
  type    = list(string)
  default = ["grype"]
  description = "List of packages to install via apk"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "googlecompute" "chainguard" {
  communicator  = "ssh"
  ssh_username  = "root"
  ssh_file_transfer_method = "sftp" # scp

  project_id    = var.project_id
  source_image  = var.source_image
  zone          = var.zone
  image_name    = "chainguard-custom-${local.timestamp}"
  image_description = "Custom Chainguard image built with Packer"
  machine_type  = "e2-standard-2"

  // Optional: Set service account or scopes if needed
  // service_account_email = "your-service-account@your-project.iam.gserviceaccount.com"
  scopes = [
    "https://www.googleapis.com/auth/cloud-platform"
  ]

  // Set metadata
  metadata = {
    created-by = "packer"
    project    = "ami-creation"
    environment = "development"
    temporary  = "yes"
  }

  // Set labels (equivalent to tags in AWS)
  image_labels = {
    name        = "chainguard-custom"
    environment = "development"
    builder     = "packer"
  }

  // Optional: Disk settings
  disk_size = 10
  disk_type = "pd-standard"
}

build {
  sources = ["source.googlecompute.chainguard"]

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
