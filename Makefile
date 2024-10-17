all: kernel initrd disk

wolfi-vm: wolfi-vm-ephemeral

wolfi-vm-ephemeral: kernel initrd
	./wolfi-vm -e

wolfi-vm-disk: kernel initrd disk
	./wolfi-vm -d disk.generic.x86_64.raw

disk: kernel initrd
	ls disk*generic* 2>/dev/null || docker run --privileged --rm -it \
		-e HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" \
		--mount type=bind,source="./",destination="/srv" \
		cgr.dev/chainguard/wolfi-base:latest \
		/srv/.mkdiskimg $(shell id -u):$(shell id -g) /srv/$(shell ls vmlinuz* | sort -V | tail -n1) /srv/$(shell ls initrd* | sort -V | tail -n1)
		[ -f "disk.raw" ] && mv -f disk.raw disk.generic.$(shell uname -m).raw || true

kernel:
	ls vmlinuz* 2>/dev/null || ./.fetch-linux-kernel

initrd:
	ls initrd*gz 2>/dev/null || docker run --rm -it \
		-e HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" \
		-e SSHKEY="$(shell cat "${SSHKEY}" | base64 -w0)" \
		--mount type=bind,source="./",destination="/srv" \
		cgr.dev/chainguard/wolfi-base:latest \
		/srv/.mkinitrd $(shell id -u):$(shell id -g) $(shell uname -m)

initrd-image:
	cosign version >/dev/null || exit 127
	# jq our way to have a valid apko manifest for input image
	 cosign verify-attestation  \
		--type https://apko.dev/image-configuration \
		--certificate-oidc-issuer https://token.actions.githubusercontent.com  \
		--certificate-identity https://github.com/chainguard-images/images-private/.github/workflows/release.yaml@refs/heads/main \
		"cgr.dev/chainguard-private/${DOCKER_IMAGE}"  | jq -r .payload | base64 -d | jq .predicate > tmp-manifest.json || exit 1

	# same as .mkinird but we pass a manifest and a cloud provider to behave accordingly
	ls initrd*gz 2>/dev/null || docker run --rm -it \
		-e HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" \
		-e SSHKEY="$(shell cat "${SSHKEY}" | base64 -w0)" \
		--mount type=bind,source="./",destination="/srv" \
		cgr.dev/chainguard/wolfi-base:latest \
		/srv/.mkinitrd-container $(shell id -u):$(shell id -g) $(shell uname -m) "/srv/tmp-manifest.json"

disk-image: initrd-image
	@if ! ls disk*${DOCKER_IMAGE}* 2>/dev/null; then\
		docker run --privileged --rm -it \
			-e HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" \
			--mount type=bind,source="./",destination="/srv" \
			cgr.dev/chainguard/wolfi-base:latest \
			/srv/.mkdiskimg $(shell id -u):$(shell id -g) /srv/$(shell ls vmlinuz* | sort -V | tail -n1) /srv/$(shell ls initrd* | sort -V | tail -n1);\
		mv disk.raw disk.${DOCKER_IMAGE}.$(shell uname -m).raw;\
	fi

disk-image-clean:
	rm -f initrd*gz vmlinuz* disk*${DOCKER_IMAGE}*
	$(MAKE) disk-image

aws-image-upload:
	# aws configure sso --profile cg-dev
	rm -f /tmp/import-task.json
	$(eval filename := $(shell ls -1 chainguard-${DOCKER_IMAGE}*aws*.vhd | tail -1))
	$(eval imagename := $(shell ls -1 chainguard-${DOCKER_IMAGE}*aws*.vhd | tail -1 | sed 's|\.tar\.gz||g'))
	aws s3 cp --profile cg-dev "${filename}"  s3://wolfi-vm-images-workloads/
	aws ec2 import-snapshot \
		--profile cg-dev \
		--description "${filename}" \
		--disk-container "file://./${filename}.json" | tee /tmp/import-task.json
	$(eval taskid := $(shell jq -r '.ImportTaskId' /tmp/import-task.json))
	echo ${taskid}
	$(eval status := $(shell aws ec2 describe-import-snapshot-tasks --profile cg-dev --region us-east-1 --import-task-ids ${taskid} | jq -r .ImportSnapshotTasks[0].SnapshotTaskDetail.Status))
	while [ "${status}" != "completed" ]; do \
		@echo "importing image..."; \
		sleep 10; \
		$(eval status := $(shell  aws ec2 describe-import-snapshot-tasks --profile cg-dev --region us-east-1 --import-task-ids ${taskid} | jq -r .ImportSnapshotTasks[0].SnapshotTaskDetail.Status)) \
	done; \
	echo "importing done, creating AMI"
	$(eval snapshotid := $(shell  aws ec2 describe-import-snapshot-tasks --profile cg-dev --region us-east-1 --import-task-ids ${taskid} | jq -r .ImportSnapshotTasks[0].SnapshotTaskDetail.SnapshotId))
	aws ec2 register-image \
		--profile cg-dev \
		--name "${imagename}" \
		--description "${imagename}" \
		--block-device-mappings DeviceName="/dev/sda1",Ebs={SnapshotId="${snapshotid}"} \
		--root-device-name "/dev/sda1" \
		--boot-mode "uefi"


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
	rm -f chainguard-*.vhd* disk* *.gz vmlinuz*
