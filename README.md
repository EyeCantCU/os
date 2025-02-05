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

## TODO

- a way to validate a created image
- a way to convert to other formats
