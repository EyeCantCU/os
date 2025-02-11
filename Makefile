TOP_D := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
TOOLS_D = $(TOP_D)/tools

# when converting from an existing image, we stuff these in.
BOOT_PKGS = linux-boot-configuration mattmoor-chainit-init
ALL_DISKS = generic google docker-runner workstation aws-ec2

# Darwin reports arm64 for 'uname -m'
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_M),arm64)
UNAME_M = aarch64
endif

BUILDER_ARCH ?= $(UNAME_M)
ARCH ?= $(BUILDER_ARCH)

ARCH_OUT_D = output/$(ARCH)

BUILDER_KERNEL := builder/kernel-$(BUILDER_ARCH)
BUILDER_INITRD := builder/initrd-$(BUILDER_ARCH)
BUILDER_DEBUG_INITRD := builder/initrd-debug-$(BUILDER_ARCH)

cfgs = $(wildcard configs/*.yaml)
# names is a list of each basename cfg
names = $(foreach cfg,$(cfgs),$(subst .yaml,,$(notdir $(cfg))))

gosrc := $(shell find main.go pkg/ -name "*.go")
apkoaas: $(gosrc)
	go build -o apkoaas

test:
	go test -v -tags withauth ./...

.PHONY: disks
disks: $(foreach name,$(ALL_DISKS),disk-$(name))

# disk-generic depends on ARCH_OUT_D/generic/disk.raw
disk_targets = $(foreach name,$(names),disk-$(name))
.PHONY: $(disk_targets)
$(disk_targets): disk-%: $(ARCH_OUT_D)/%/disk.raw

disk_debug_targets = $(foreach name,$(names),disk-debug-$(name))
.PHONY: $(disk_debug_targets)
$(disk_debug_targets): disk-debug-%: $(ARCH_OUT_D)/%/disk-debug.raw

run_targets = $(foreach name,$(names),run-$(name))
.PHONY: $(run_targets)
$(run_targets): run-%: $(ARCH_OUT_D)/%/disk.raw builder/ovmf-$(ARCH).fd
	$(TOP_D)/apkoaas debug --arch=$(ARCH) --ovmf=builder/ovmf-$(ARCH).fd $(ARCH_OUT_D)/$*/disk.raw

run_debug_targets = $(foreach name,$(names),run-debug-$(name))
.PHONY: $(run_debug_targets)
$(run_debug_targets): run-debug-%: $(ARCH_OUT_D)/%/disk-debug.raw builder/ovmf-$(ARCH).fd
	$(TOP_D)/apkoaas debug --arch=$(ARCH) --ovmf=builder/ovmf-$(ARCH).fd $(ARCH_OUT_D)/$*/disk-debug.raw

debug_shell_targets = $(foreach name,$(names),debug-shell-$(name))
.PHONY: $(debug_shell_targets)
$(debug_shell_targets): debug-shell-%:
	@echo "::: Make sure you have a 'run-debug-$*' session running or this wont work"
	@echo "[hit enter]"
	@socat STDIO,cfmakeraw,isig=1 UNIX:$(ARCH_OUT_D)/$(subst .yaml,,$*)/disk-debug.raw.socket

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


.PHONY: builder
builder: $(BUILDER_KERNEL) $(BUILDER_INITRD)

builder/initrd-debug-%: apkoaas iac/builder-debug.yaml
	$(TOP_D)/apkoaas make-builder --arch=$* iac/builder-debug.yaml $@

builder/initrd-%: apkoaas iac/builder.yaml
	@mkdir -p $(dir $@)
	$(TOP_D)/apkoaas make-builder --arch=$* iac/builder.yaml $@

builder/ovmf-%.fd: apkoaas
	$(TOP_D)/apkoaas fetch --arch=$* ovmf $@

builder/kernel-%: apkoaas
	$(TOP_D)/apkoaas fetch --arch=$* kernel $@

$(ARCH_OUT_D)/%/disk.raw: configs/%.yaml apkoaas $(BUILDER_KERNEL) $(BUILDER_INITRD)
	@mkdir -p $(dir $@)
	$(TOP_D)/apkoaas build \
		--arch=$(ARCH) \
		--build-arch=$(BUILDER_ARCH) \
		--builder-cpio=$(BUILDER_INITRD) \
		--kernel=$(BUILDER_KERNEL) \
		--output=$@ \
		configs/$*.yaml

$(ARCH_OUT_D)/%/disk-debug.raw: apkoaas $(BUILDER_KERNEL) $(BUILDER_DEBUG_INITRD)
	@mkdir -p $(dir $@)
	$(TOP_D)/apkoaas build \
		--arch=$(ARCH) \
		--build-arch=$(BUILDER_ARCH) \
		--builder-cpio=$(BUILDER_DEBUG_INITRD) \
		--kernel=$(BUILDER_KERNEL) \
		--output=$@ \
		configs/$*.yaml "$@"

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

show-vars:
	@echo UNAME_M=$(UNAME_M)
	@echo BUILDER_ARCH=$(BUILDER_ARCH)
	@echo ARCH=$(ARCH)
	@echo ARCH_OUT_D=$(ARCH_OUT_D)
	@echo BUILDER_KERNEL=$(BUILDER_KERNEL)
	@echo BUILDER_INITRD=$(BUILDER_INITRD)
	@echo ALL_DISKS=$(ALL_DISKS)
	@echo names=$(names)

.PRECIOUS: $(foreach bname,disk.raw disk-debug.raw image.tar initramfs.cpio,$(ARCH_OUT_D)/%/$(bname))
.PRECIOUS: configs/%.yaml builder/ovmf-%.fd

arches = aarch64 x86_64
ifneq ($(filter-out $(arches),$(BUILDER_ARCH)),)
$(error BUILDER_ARCH '$(BUILDER_ARCH)' not supported. Must be one of $(arches))
endif
ifneq ($(filter-out $(arches),$(ARCH)),)
$(error ARCH '$(ARCH)' not supported. Must be one of $(arches))
endif
