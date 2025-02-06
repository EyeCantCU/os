TOP_D := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
TOOLS_D = $(TOP_D)/tools
# when converting from an existing image, we stuff these in.
BOOT_PKGS ?= linux-boot-configuration mattmoor-chainit-init
ALL_DISKS := generic google docker-runner workstation aws-ec2

ARCHES := aarch64 x86_64

# Darwin reports arm64 for 'uname -m'
ARCH ?= $(shell uname -m)
ifeq ($(ARCH),arm64)
ARCH=aarch64
endif

ifneq ($(filter-out $(ARCHES),$(ARCH)),)
$(error ARCH '$(ARCH)' not supported. Must be one of $(ARCHES))
endif

ARCH_OUT_D = output/$(ARCH)

gosrc := $(shell find main.go pkg/ -name "*.go")

apkoaas: $(gosrc)
	go build -o apkoaas

test:
	go test -v -tags withauth ./...

.PHONY: disks
disks: $(foreach name,$(ALL_DISKS),disk-$(name))

.PHONY: debug-shell-%
debug-shell-%:
	@echo "::: Make sure you have a 'debug-disk-$*' session running or this wont work"
	@echo "[hit enter]"
	@socat STDIO,cfmakeraw,isig=1 UNIX:$(ARCH_OUT_D)/$(subst .yaml,,$*)/disk.raw.socket

configs/%.yaml:
	@mkdir -p $(dir $@)
	cosign verify-attestation \
		--type=https://apko.dev/image-configuration \
		--certificate-oidc-issuer=https://token.actions.githubusercontent.com \
		--certificate-identity=https://github.com/chainguard-images/images-private/.github/workflows/release.yaml@refs/heads/main \
		"cgr.dev/chainguard-private/$*" > $@.tmp.json
	jq -r . $@.tmp.json | yq -P > $@.tmp && rm $@.tmp.json
	yq -P -i '.payload | @base64d | fromjson | .predicate' $@.tmp
	for i in $(BOOT_PKGS) ; do \
		yq -i ".contents.packages += [\"$$i\"]" $@.tmp; \
	done
	mv $@.tmp $@

builder: apkoaas
	@[ -f $(ARCH_OUT_D)/builder/initramfs.cpio ] || ./apkoaas make-builder iac/builder.yaml

builder-debug: apkoaas
	@[ -f $(ARCH_OUT_D)/builder-debug/initramfs.cpio ] || ./apkoaas make-builder iac/builder-debug.yaml

debug-disk-%: apkoaas builder-debug
	@mkdir -p $(ARCH_OUT_D)/$(subst .yaml,,$*)/
	$(TOP_D)/apkoaas build \
		--arch $(ARCH) \
		--builder-cpio $(ARCH_OUT_D)/builder-debug/initramfs.cpio \
		--kernel $(ARCH_OUT_D)/builder-debug/kernel-$(ARCH) \
		$(TOP_D)/configs/$*.yaml
	./apkoaas debug --arch $(ARCH) --ovmf $(ARCH_OUT_D)/builder-debug/ovmf-$(ARCH).fd $(ARCH_OUT_D)/$(subst .yaml,,$*)/disk.raw

disk-%: apkoaas builder
	@mkdir -p $(ARCH_OUT_D)/$(subst .yaml,,$*)/
	$(TOP_D)/apkoaas build \
		--arch $(ARCH) \
		--builder-cpio $(ARCH_OUT_D)/builder/initramfs.cpio \
		--kernel $(ARCH_OUT_D)/builder/kernel-$(ARCH) \
		$(TOP_D)/configs/$*.yaml

%.vmdk: %.raw
	qemu-img convert -O vmdk -o subformat=streamOptimized $< $@

# TODO add awspub installation
# TODO build arm64
awspub: disk-aws-ec2 output/x86_64/aws-ec2/disk.vmdk
	:> .awspub.mapping
	echo '---' >> .awspub.mapping
	echo "serial: $$(date +%s)" >> .awspub.mapping
	echo "employee: $$(id -nu)" >> .awspub.mapping
	aws sso login
	awspub create --config-mapping .awspub.mapping tools/awspub-config.yaml

clean:
	rm -Rf output builder apkoaas *.raw

.PRECIOUS: $(foreach bname,disk.raw disk-debug.raw image.tar initramfs.cpio,$(ARCH_OUT_D)/%/$(bname))
.PRECIOUS: configs/%.yaml builder/ovmf-%.fd
