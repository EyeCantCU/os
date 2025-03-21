packer {
  required_plugins {
    azure = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/azure"
    }
  }
}

variable "source_image_offer" {
  type    = string
  description = "The source image offer to customize"
}

variable "source_image_publisher" {
  type    = string
  description = "The source image publisher"
}

variable "source_image_sku" {
  type    = string
  default = "latest"
  description = "The source image SKU"
}

variable "location" {
  type    = string
  default = "eastus"
  description = "The Azure region to build the image in"
}

variable "vm_size" {
  type    = string
  default = "Standard_B2s"
  description = "The Azure VM size"
}

variable "subscription_id" {
  type    = string
  description = "The Azure subscription ID"
}

variable "resource_group_name" {
  type    = string
  description = "The Azure resource group name where the image will be stored"
}

variable "packages" {
  type    = list(string)
  default = ["grype"]
  description = "List of packages to install via apk"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
  image_name = "chainguard-custom-${local.timestamp}"
}

source "azure-arm" "chainguard" {
  // Authentication - Use Azure CLI auth or set service principal credentials as env vars
  // Other auth options: managed_identity_*
  subscription_id = var.subscription_id

  // VM Configuration
  os_type                   = "Linux"
  image_offer               = var.source_image_offer
  image_publisher           = var.source_image_publisher
  image_sku                 = var.source_image_sku
  communicator              = "ssh"
  ssh_username              = "root"
  ssh_file_transfer_method  = "sftp"

  // Resource Configuration
  location                  = var.location
  vm_size                   = var.vm_size
  resource_group_name       = var.resource_group_name

  // Managed Image Configuration
  managed_image_name        = local.image_name
  managed_image_resource_group_name = var.resource_group_name

  // Azure Tags (equivalent to GCP labels)
  azure_tags = {
    created_by   = "packer"
    project      = "ami-creation"
    environment  = "development"
    builder      = "packer"
  }

  // Disk Configuration
  os_disk_size_gb = 30
}

build {
  sources = ["source.azure-arm.chainguard"]

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

  // Cleanup script is often required for Azure to properly deprovision the VM
  provisioner "shell" {
    execute_command = "chmod +x {{ .Path }}; {{ .Vars }} sudo -E sh '{{ .Path }}'"
    inline = [
      "echo 'Performing Azure VM deprovision'",
      "/usr/sbin/waagent -force -deprovision+user && export HISTSIZE=0 && sync"
    ]
    inline_shebang = "/bin/sh -x"
  }

  // Optionally add more customizations
  post-processor "manifest" {
    output = "manifest.json"
    strip_path = true
  }
}
