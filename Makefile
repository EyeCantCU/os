TOP_D := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
TOOLS_D = $(TOP_D)/tools

# when converting from an existing image, we stuff these in.
BOOT_PKGS = linux-boot-configuration mattmoor-chainit-init

disks_aws = aws-base aws-agents aws-docker aws-docker-dev aws-eks-dev
disks_gcp = gcp-base gcp-agents gcp-agents-docker-dev gcp-docker gcp-docker-dev
disks_qemu = generic
disks_azure = azure-base azure-agents azure-aks-dev azure-eap-dev
disks_workstation = workstation

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

.PHONY: disks-aws disks-azure disks-gcp disks-qemu disks-workstation
disks-aws: $(foreach name,$(disks_aws),disk-$(name))
disks-azure: $(foreach name,$(disks_azure),disk-$(name))
disks-gcp: $(foreach name,$(disks_gcp),disk-$(name))
disks-qemu: $(foreach name,$(disks_qemu),disk-$(name))
disks-workstation: $(foreach name,$(disks_workstation),disk-$(name))

.PHONY: list-aws list-azure list-gcp list-qemu list workstation
list-all:
	@for n in $(names); do echo $$n; done
list-aws:
	@for n in $(disks_aws); do echo $$n; done
list-azure:
	@for n in $(disks_azure); do echo $$n; done
list-gcp:
	@for n in $(disks_gcp); do echo $$n; done
list-qemu:
	@for n in $(disks_qemu); do echo $$n; done
list-workstation:
	@for n in $(disks_workstation); do echo $$n; done

.PHONY: disks
disks: $(foreach name,$(names),disk-$(name))

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
	$(TOP_D)/apkoaas --log-level=debug make-builder --arch=$* iac/builder-debug.yaml $@

builder/initrd-%: apkoaas iac/builder.yaml
	@mkdir -p $(dir $@)
	$(TOP_D)/apkoaas make-builder --arch=$* iac/builder.yaml $@

builder/ovmf-%.fd: apkoaas
	$(TOP_D)/apkoaas fetch --arch=$* ovmf $@

builder/kernel-%: apkoaas
	$(TOP_D)/apkoaas fetch --arch=$* kernel $@

%.vmdk: %.raw
	./tools/aws-image-upload create-vmdk $< $@

PUBLISH_TARGET ?= dev
COMMIT ?= $(shell git rev-parse HEAD || echo no-git)
PREFIX ?= $(shell id -un)
BUILD_TIMESTAMP ?= $(shell date --utc "+%Y%m%d-%H%M")
AZVERSION = $(shell BUILD_TIMESTAMP="$(BUILD_TIMESTAMP)"; echo "$${BUILD_TIMESTAMP%-*}.$${BUILD_TIMESTAMP#*-}.0")
AZTAGS += env=$(PUBLISH_TARGET) commit=$(COMMIT)

ifeq ($(COMMIT),$(filter $(COMMIT), "", no-git))
$(error "Bad value for COMMIT: '$(COMMIT)'")
endif

ifeq ($(AZVERSION),$(filter $(AZVERSION), "", ..0))
$(error "Bad value for AZVERSION: '$(AZVERSION)')
endif

ifeq ($(ARCH),aarch64)
AWSARCH = arm64
AZARCH = arm64
else
AWSARCH = x86_64
AZARCH = x64
endif
ifeq ($(PUBLISH_TARGET),dev)
AZGALLERY = vmtesting_dev
else ifeq ($(PUBLISH_TARGET),staging)
AZGALLERY = vmtesting
else ifeq ($(PUBLISH_TARGET),eap)
AZGALLERY = chainguard_vms_eap
else
$(error "Bad value for PUBLISH_TARGET: '$(PUBLISH_TARGET)')
endif

awspub-%: AWSSTEM=$(subst awspub-aws-,,$@)
awspub-%: AWSNAME=$(PREFIX)-$(AWSSTEM)-$(AWSARCH)-$(BUILD_TIMESTAMP)
awspub-%: AWSSSM=$(PREFIX)-$(AWSSTEM)-$(AWSARCH)
awspub-%: $(ARCH_OUT_D)/%/disk.vmdk
	./tools/aws-image-upload --name=$(AWSNAME) --arch=$(AWSARCH) $(if $(SSM),--ssm=$(AWSSSM)) $(if $(SHARE),--share="$(SHARE)") $< $(BUCKET)

.PHONY: awspub
awspub: $(foreach name,$(disks_aws),awspub-$(name))

.PHONY: aws-create-% aws-publish-% aws-create aws-publish
# these are just so human can type 'make aws-create-aws-base' to do the create/publish
$(foreach name,$(disks_aws),aws-create-$(name)): aws-create-%: $(ARCH_OUT_D)/awspub/create/%.json
$(foreach name,$(disks_aws),aws-publish-$(name)): aws-publish-%: $(ARCH_OUT_D)/awspub/publish/%.output
aws-publish: $(foreach name,$(disks_aws),aws-publish-$(name))
aws-create: $(foreach name,$(disks_aws),aws-create-$(name))

output/awspub.mapping:
	mkdir -p output
	echo "---" > $@
	echo "BUILD_TIMESTAMP: $(BUILD_TIMESTAMP)" >> $@

# capture_stdout(output,command)
# safely write the output of command to output
capture_stdout = rm -f "$(1)" && mkdir -p "$(dir $(1))" && \
	tmpf="$(1).tmp.$$$$" && trap "rm -f $$tmpf" EXIT && \
	echo "$(2) > $(1)" && $(2) > "$$tmpf" && mv "$$tmpf" "$(1)" && \
	echo "== $(1) ==" && cat "$(1)" && echo

# we use the json output of awspub to indicate the thing was published.
$(ARCH_OUT_D)/awspub/create/%.json: output/awspub.mapping $(ARCH_OUT_D)/%/disk.vmdk
	@$(call capture_stdout,$@,\
	  awspub create --config-mapping=output/awspub.mapping awspub/$(ARCH)/$*.yaml)

.PHONY: publish-azure
publish-azure: $(foreach name,$(disks_azure),publish-azure-$(subst azure-,,$(name)))
$(foreach name,$(disks_azure),publish-azure-$(subst azure-,,$(name))): publish-azure-%: $(ARCH_OUT_D)/azure-%/publish.$(PUBLISH_TARGET).json

$(ARCH_OUT_D)/azure-%/publish.$(PUBLISH_TARGET).json: AZNAME=$(PREFIX)-$*-$(AZARCH)
$(ARCH_OUT_D)/azure-%/publish.$(PUBLISH_TARGET).json: $(ARCH_OUT_D)/azure-%/disk.raw
	@$(call capture_stdout,$@,\
		$(TOOLS_D)/azure-image-upload --arch=$(AZARCH) --gallery=$(AZGALLERY) \
			--name=$(AZNAME) --disk-name=$(AZNAME)-$(BUILD_TIMESTAMP) --image-version=$(AZVERSION) \
			--tags="$(AZTAGS)" $(dir $@)disk.raw)
	#
# we use the stdout of awspub publish to indicate the thing was published.
$(ARCH_OUT_D)/awspub/publish/%.output: output/awspub.mapping $(ARCH_OUT_D)/awspub/create/%.json
	@$(call capture_stdout,$@,\
	  awspub publish --config-mapping=output/awspub.mapping awspub/$(ARCH)/$*.yaml)

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
		--log-level=debug \
		--arch=$(ARCH) \
		--build-arch=$(BUILDER_ARCH) \
		--builder-cpio=$(BUILDER_DEBUG_INITRD) \
		--kernel=$(BUILDER_KERNEL) \
		--output=$@ \
		configs/$*.yaml "$@"

clean:
	rm -Rf output builder apkoaas *.raw

install-deps:
	sudo sh -c 'apt-get --quiet update && \
	  apt-get --quiet --assume-yes \
	    --option=Dpkg::Options::=--force-confold \
	    --option=Dpkg::options::=--force-unsafe-io \
	    install --no-install-recommends \
	      cpu-checker parallel python3-venv qemu-system-x86 qemu-utils'
	kvm-ok

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
.PRECIOUS: %.vmdk

arches = aarch64 x86_64
ifneq ($(filter-out $(arches),$(BUILDER_ARCH)),)
$(error BUILDER_ARCH '$(BUILDER_ARCH)' not supported. Must be one of $(arches))
endif
ifneq ($(filter-out $(arches),$(ARCH)),)
$(error ARCH '$(ARCH)' not supported. Must be one of $(arches))
endif

.PHONY: convert
convert:
	./hack/convert.sh
