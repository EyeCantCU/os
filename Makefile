.PHONY: all kernel clean aws-image-upload google-image-upload
all: disk

wolfi-vm: wolfi-vm-ephemeral

wolfi-vm-ephemeral: kernel initrd
	./wolfi-vm -e

wolfi-vm-disk: initrd disk
	./wolfi-vm -d disk.generic.x86_64.raw

kernel:
	ls vmlinuz* 2>/dev/null || ./.fetch-linux-kernel

docker-runner-image:
	DOCKER_IMAGE=docker-runner make disk

generic-image:
	DOCKER_IMAGE=generic make disk

initrd:
	rm -f tmp-manifest*.json
	cosign version >/dev/null || exit 127
	# jq our way to have a valid apko manifest for input image
	set -o pipefail; cosign verify-attestation  \
		--type https://apko.dev/image-configuration \
		--certificate-oidc-issuer https://token.actions.githubusercontent.com  \
		--certificate-identity https://github.com/chainguard-images/images-private/.github/workflows/release.yaml@refs/heads/main \
		"cgr.dev/chainguard-private/${DOCKER_IMAGE}"  | jq -r .payload | base64 -d | jq .predicate > tmp-manifest.json || rm -f tmp-manifest.json

	ls tmp-manifest.json ||  cp -f "${DOCKER_IMAGE}".json tmp-manifest.json

	# same as .mkinird but we pass a manifest and a cloud provider to behave accordingly
	ls initrd*gz 2>/dev/null || docker run --rm -it \
		-e HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" \
		-e SSHKEY="$(shell cat "${SSHKEY}" | base64 -w0)" \
		--mount type=bind,source="./",destination="/srv" \
		cgr.dev/chainguard/wolfi-base:latest \
		/srv/.mkinitrd $(shell id -u):$(shell id -g) $(shell uname -m) "/srv/tmp-manifest.json"

disk: initrd
	@if ! ls disk*${DOCKER_IMAGE}* 2>/dev/null; then\
		docker run --privileged --rm -it \
			-e DOCKER_IMAGE=${DOCKER_IMAGE} \
			-e HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" \
			--mount type=bind,source="./",destination="/srv" \
			cgr.dev/chainguard/wolfi-base:latest \
			/srv/.mkdiskimg $(shell id -u):$(shell id -g) /srv/$(shell ls vmlinuz* | sort -V | tail -n1) /srv/$(shell ls initrd* | sort -V | tail -n1);\
		mv disk.raw disk.${DOCKER_IMAGE}.$(shell uname -m).raw;\
	fi

aws-image-upload:
	# aws configure sso --profile cg-dev
	./aws-image-upload "${DOCKER_IMAGE}"

google-image-upload:
	$(eval filename := $(shell ls -1 chainguard-${DOCKER_IMAGE}*.tar.gz | tail -1))
	$(eval imagename := $(shell ls -1 chainguard-${DOCKER_IMAGE}*.tar.gz | tail -1 | sed 's|\.tar\.gz||g'))
	gcloud storage cp ${filename} gs://wolfi-vm-images-workloads/
	gcloud compute images create \
		${imagename} \
		--project=wolfi-vm \
		--source-uri=https://storage.googleapis.com/wolfi-vm-images-workloads/${filename} \
		--guest-os-features=UEFI_COMPATIBLE,VIRTIO_SCSI_MULTIQUEUE

clean:
	rm -f chainguard-*.vhd* disk* *.gz vmlinuz* tmp-manifest*.json
