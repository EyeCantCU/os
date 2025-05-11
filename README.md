# Build your own VM
Chainguard Virtual Machines are build in a very similar fashion to how containers are built. Apko creates a tarball that is then converted into a raw disk image.

## Create your VM Config file
A VM config file is just a apko YAML file that describes the packages that are part of your virtual machine images.

These files are stored under `configs` folder.

## Build your VM
Once you have created a config, you can run the following command:

```
make disk-<your config>
```

For example, the following command will build a raw disk image from the `configs/generic.yaml` file.

```
make disk-generic
```

## Running your VM
You can test run your VM with:

```
make run-<your config>
```

Or for example:

```
make run-generic
```

## Accessing your VM
VM images come with no default user or password. If you wish to test it locally and access it, you will need to backdoor it. You can do so with `tools/backdoor-image`.

For example:

```
sudo ./tools/backdoor-image --password-auth --password <your-password> outputs/x86_64/generic/disk.raw
```


# Experiments with tar2efi go library


## Compile

```
go build .
```

## Run

```console
./wolfi-vm build --builder iac/builder.yaml ../configs/generic.yaml
```

you can specify a kernel so that it's not downloaded each time:

```console
./wolfi-vm build --builder iac/builder.yaml --kernel /boot/vmlinuz ../configs/generic.yaml
```

you can specify an arch so that it's not downloaded each time:

```console
./wolfi-vm build --builder iac/builder.yaml --arch arm64 ../configs/generic.yaml
```

Output is: `disk.raw`

## Test

`go test -tags withauth ./...`

or

`make test`

This will test the builder logic, builder entrypoint script and the tar2efi functions

## Converting existing OCI images

[convert.txt](./convert.txt) contains an allowlist of files
which will convert some of our images into VMs.

This is done by downloading the apko attestation and injecting a few extra apks, then building with `apko build-minirootfs`

Then, to build them, you can run
```
make convert
```

Then checkout the `converted/` directory.

## TODO

- a way to validate a created image
- a way to convert to other formats
