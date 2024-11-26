# Chainguard Extra Packages
## Overview
This repository provides the latest packages for software products with non-open-source licenses. These packages are designed to be used with Chainguard Images, which provide a secure and reliable base for your applications.
## Getting Started
To get started with this repository, you can use the [wolfi-base image](https://images.chainguard.dev/directory/image/wolfi-base/versions) as a starting point. This image includes the apk package manager, to facilitate adding additional software packages.

### Adding the repo to your Wolfi repositories
In order to make wolfi-base capable of pulling extra packages, you will need to perform the following steps:
```
echo "https://packages.cgr.dev/extras" >> /etc/apk/repositories
curl -o /etc/apk/keys/chainguard-extras.rsa.pub https://packages.cgr.dev/extras/chainguard-extras.rsa.pub
```
### Building an Image
To build an image using wolfi-base, follow these steps:
1. Start with the wolfi-base image
2. Use the apk package manager to add the packages you need
3. Build your image using the resulting layers

For more information on using Wolfi Images, see the [Chainguard Images documentation](https://edu.chainguard.dev/chainguard/chainguard-images/how-to-use-chainguard-images/#:~:text=Compile%20the%20dependency%20from%20source,as%20possible%20without%20sacrificing%20maintainability.).
