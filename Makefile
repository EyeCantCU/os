all: kernel initrd disk

wolfi-vm: wolfi-vm-ephemeral

wolfi-vm-ephemeral: kernel initrd
	./wolfi-vm -e

wolfi-vm-disk: kernel initrd disk
	./wolfi-vm

disk: kernel initrd
	ls disk* 2>/dev/null || docker run --privileged --rm -it \
		-e HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" \
		--mount type=bind,source="./",destination="/srv" \
		cgr.dev/chainguard/wolfi-base:latest \
		/srv/.mkdiskimg $(shell id -u):$(shell id -g) /srv/$(shell ls vmlinuz* | sort -V | tail -n1) /srv/$(shell ls initrd* | sort -V | tail -n1)

kernel:
	ls vmlinuz* 2>/dev/null || ./.fetch-linux-kernel

initrd:
	ls initrd*gz 2>/dev/null || docker run --rm -it \
		--mount type=bind,source="./",destination="/srv" \
		cgr.dev/chainguard/wolfi-base:latest \
		/srv/.mkinitrd $(shell id -u):$(shell id -g)

clean:
	rm -f disk* initrd*gz vmlinuz*
