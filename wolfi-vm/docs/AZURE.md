# AZURE_README.md

## High level image publishing flow

apko image -> Image Gallery -> Technical Plan Configuration for Marketplace Offer

## Gallery Upload Flow

### Overview

This code is in `tools/azure-image-upload`

Raw disks output from our builds are converted to VHDs and expanded to a
reasonable size dictated by the Azure certification recommendations.

A remote disk is created and marked writeable after which we copy the contents
of our raw disk to the remote disk.  This disk is then closed to finalize the
write.

If a new image definition is being created it's created. A new image version is
created using the uploaded disk as the OS disk for that image.

This is different from the flow that most of the docs and tutorials online do.
The majority of people are customizing existing images.  Their process typically
involves booting a VM making the desired changes and generalizing the instance
by removing specific data. Microsoft provides several methods for doing this.
Since we never boot the VM we never need to generalize them and our image
creation process is a bit more straightforward.

### Permissions

Uploaders need permission to create, mark writeable and close disks. As well as
add image definitions and version to whatever galleries in whatever resources
groups they galleries are in. Contributor permission on the target resource
groups and reader elsewhere should be sufficient.

### docs

- https://learn.microsoft.com/en-us/azure/virtual-machines/linux/tutorial-custom-images


## PartnerCenter Upload Flow

### Overview

Microsoft PartnerCenter is where you go to assemble Offers to present to various
microsoft commercial marketplaces. It is not part of Azure. It is unrelated to
the Azure Private Marketplace.

Inside of the Marketplace we need to interact with Offers and plans. Offers are
what we're selling. If we're selling a "Chainguard VM" then that can be the offer.
The Plans are the options for buying. Subscriptions pricing options, bulk pricing
etc. Technical Configuration of the the Plan is the details for what you're
buying. For example providing x86_64 and arm64. The part we care about most
here is the Technical Configurations of plans.

Unlike Shared Image Galleries the Technical Configuration can associated multiple
images to a single image version. When we add an virtual machine image version
to a Technical Configuration we add the same version from our galleries for both
architectures.

This code is in `tools/azure-marketplace-add-vm-image-version`.

A json snippet with the correct image version definitions is created. This  is
assembled with a few variables passed from the environment and the commandline.

This new image version is then appended to the existing definition and the
Technical Configuration is updated.

The code for updating the image is https://github.com/justinvreeland/partnercenter-cli-extension/tree/draft-vm-image-version

You can see discussion and alternatives in https://github.com/chainguard-dev/internal-dev/issues/10427

### Permissions

#### Microsoft

PartnerCenter needs permission to acquire the VM resources from the target
subscription to do this. To do this you need to register the Partner Center
Ingestion provider.
`az provider register --namespace Microsoft.PartnerCenterIngestion`
This principal then needs to be assigned the roles as `Computer Gallery Image Reader`.

#### Chainguard

There is no terraform provider for PartnerCenter that I can find. The APIs and
SDKs seem to be deprecated and unsupported. Those that aren't don't provide user
management and focus more on managing plans and configurations or customer
management and analytics.

For our purposes we need a service principal that can has read access to the
galleries we'll want to add images from. And has "Developer" permissions in
PartnerCenter.


### docs
- https://learn.microsoft.com/en-us/partner-center/marketplace-offers/azure-vm-plan-technical-configuration
- https://learn.microsoft.com/en-us/partner-center/marketplace-offers/azure-vm-use-own-image#provide-partner-center-with-permission-to-your-azure-compute-gallery
