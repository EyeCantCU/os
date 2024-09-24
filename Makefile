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
		/srv/.mkinitrd $(shell uname -m) $(shell id -u):$(shell id -g)

initrd-image:
	# get the plan from our image repo, locally and export to json
	([ ! -e ${IMAGE_REPO}/mega-module.tfplan ] && cd ${IMAGE_REPO} && make write-plan || :)
	# jq our way to have a valid apko manifest for input image
	jq -r  '.planned_values.root_module.child_modules[] | select(.address=="module.${DOCKER_IMAGE}") | .child_modules[].child_modules[].resources[] | select(.type=="apko_build") | .values.config' ${IMAGE_REPO}/mega-module.tfplan.json | \
		jq --slurp -rM .[0] > tmp-manifest.json
	# same as .mkinird but we pass a manifest and a cloud provider to behave accordingly
	ls initrd*gz 2>/dev/null || docker run --rm -it \
		-e HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" \
		-e SSHKEY="${SSHKEY}" \
		--mount type=bind,source="./",destination="/srv" \
		cgr.dev/chainguard/wolfi-base:latest \
		/srv/.mkinitrd-container $(shell uname -m) $(shell id -u):$(shell id -g) "/srv/tmp-manifest.json" "${CLOUD}" && rm -f tmp-manifest*

disk-image: kernel initrd-image
	@if ! ls disk*${DOCKER_IMAGE}* 2>/dev/null; then\
		docker run --privileged --rm -it \
			-e HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" \
			--mount type=bind,source="./",destination="/srv" \
			cgr.dev/chainguard/wolfi-base:latest \
			/srv/.mkdiskimg $(shell id -u):$(shell id -g) /srv/$(shell ls vmlinuz* | sort -V | tail -n1) /srv/$(shell ls initrd* | sort -V | tail -n1);\
		mv disk.$(shell uname -m).img disk.${DOCKER_IMAGE}.$(shell uname -m).img;\
		rm -f disk.$(shell uname -m).img.tar.gz;\
	fi

disk-image-clean:
	rm -f initrd*gz vmlinuz* disk*${DOCKER_IMAGE}*
	$(MAKE) disk-image

clean:
	rm -f disk* initrd*gz vmlinuz*
