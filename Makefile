TOP_D := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
TOOLS_D = $(TOP_D)/tools
HASH := \#

# when converting from an existing image, we stuff these in.
BOOT_PKGS = linux-qemu-generic-boot-installed mattmoor-chainit-init

# list_cloud_images(cloud)
define list_cloud_images
	$(notdir $(wildcard configs/$1-*))
endef

disks_aws := $(call list_cloud_images,aws)
disks_gcp := $(call list_cloud_images,gcp)
disks_qemu := $(call list_cloud_images,generic)
disks_azure := $(call list_cloud_images,azure)
disks_vmware := $(call list_cloud_images,vmware)
disks_rpi := $(call list_cloud_images,rpi)

group_aws_noneks := $(filter-out aws-eks-%,$(call list_cloud_images,aws))
group_aws_eks := $(filter aws-eks-%,$(call list_cloud_images,aws))

# Darwin reports arm64 for 'uname -m'
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_M),arm64)
  UNAME_M = aarch64
endif

BUILDER ?= builder
BUILDER_ARCH ?= $(UNAME_M)
ARCH ?= $(BUILDER_ARCH)

ARCH_OUT_D = output/$(ARCH)

BUILDER_KERNEL := builder/kernel-$(BUILDER_ARCH)
BUILDER_INITRD := builder/initrd-$(BUILDER_ARCH)
BUILDER_DEBUG_INITRD := builder/initrd-debug-$(BUILDER_ARCH)

cfgs = $(wildcard configs/*)
# names is a list of each basename cfg
names = $(foreach cfg,$(cfgs),$(notdir $(cfg)))

gosrc := $(shell find main.go pkg/ -name "*.go")
apkoaas: $(gosrc)
	go build -o apkoaas

.PHONY: test test-generic

test:
	go test -v -tags withauth ./...

test-generic: $(ARCH_OUT_D)/generic/disk.raw builder/ovmf-$(ARCH).fd
	$(MAKE) -C vms-test TEST_ARCHES="$(ARCH)" runner/qemu tests
	QEMU_VMS=generic ./vms-test/helpers/test-wolfi-vm \
  --test-arch="$(ARCH)" --wolfi-vm="$(TOP_D)" qemu "$(TOP_D)/test-results/$(ARCH)"

.PHONY: disks-aws disks-azure disks-gcp disks-qemu disks-vmware disks-rpi
disks-aws: $(foreach name,$(disks_aws),disk-$(name))
disks-aws-eks: $(foreach name,$(group_aws_eks),disk-$(name))
disks-aws-noneks: $(foreach name,$(group_aws_noneks),disk-$(name))
disks-azure: $(foreach name,$(disks_azure),disk-$(name))
disks-gcp: $(foreach name,$(disks_gcp),disk-$(name))
disks-qemu: $(foreach name,$(disks_qemu),disk-$(name))
disks-vmware: $(foreach name,$(disks_vmware),disk-$(name))
disks-rpi: $(foreach name,$(disks_rpi),disk-$(name))

.PHONY: list list-all list-aws list-azure list-gcp list-qemu list-vmware list-rpi
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
list-vmware:
	@for n in $(disks_vmware); do echo $$n; done
list-rpi:
	@for n in $(disks_rpi); do echo $$n; done
list-%:
	@groups="$(group_$(subst -,_,$(*)))"; \
	[ -n "$$groups" ] || { echo "no group $*"; exit 1; }; \
	for n in $${groups}; do echo $$n; done

.PHONY: disks
disks: $(foreach name,$(names),disk-$(name))

# disk-generic depends on ARCH_OUT_D/generic/disk.raw
disk_targets = $(foreach name,$(names),disk-$(name))
.PHONY: $(disk_targets)
$(disk_targets): disk-%: $(ARCH_OUT_D)/%/disk.raw

qcow_targets = $(foreach name,$(names),qcow-$(name))
.PHONY: $(qcow_targets)
$(qcow_targets): qcow-%: $(ARCH_OUT_D)/%/disk.qcow2

vmdk_targets = $(foreach name,$(names),vmdk-$(name))
.PHONY: $(vmdk_targets)
$(vmdk_targets): vmdk-%: $(ARCH_OUT_D)/%/disk.vmdk

vhd_targets = $(foreach name,$(names),vhd-$(name))
.PHONY: $(vhd_targets)
$(vhd_targets): vhd-%: $(ARCH_OUT_D)/%/disk.vhd

%.qcow2: %.raw
	./tools/convert-image $< $@

# VMware-specific VMDK conversion (monolithicFlat for ESXi compatibility)
$(ARCH_OUT_D)/vmware-%/disk.vmdk: $(ARCH_OUT_D)/vmware-%/disk.raw
	./tools/convert-image --vmdk-format monolithicFlat $< $@

%.vmdk: %.raw
	./tools/convert-image $< $@

%.vhd: %.raw
	./tools/convert-image $< $@

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
	@command -v socat >/dev/null 2>&1 || { echo "$* target requires 'socat' installed." 1>&2; exit 1; }
	@echo "::: Make sure you have a 'run-debug-$*' session running or this will not work"
	@echo "[hit enter for shell prompt. ctrl-e to exit]"
	@socat STDIO,cfmakeraw,isig=1,escape=0x05 UNIX:$(ARCH_OUT_D)/$(subst .yaml,,$*)/disk-debug.raw.socket

configs/%/build.yaml:
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

builder/initrd-%: apkoaas iac/$(BUILDER).yaml
	@mkdir -p $(dir $@)
	$(TOP_D)/apkoaas make-builder --arch=$* iac/$(BUILDER).yaml $@

builder/ovmf-%.fd: apkoaas
	$(TOP_D)/apkoaas fetch --arch=$* ovmf $@

builder/kernel-%: apkoaas
	$(TOP_D)/apkoaas fetch --arch=$* kernel $@

PUBLISH_TARGET ?= dev
COMMIT ?= $(shell git rev-parse HEAD || echo no-git)
PREFIX ?= $(shell id -un)
BUILD_TIMESTAMP ?= $(shell date -u "+%Y%m%d-%H%M")
AZVERSION ?= $(shell BUILD_TIMESTAMP="$(BUILD_TIMESTAMP)"; echo "$${BUILD_TIMESTAMP%-*}.$${BUILD_TIMESTAMP$(HASH)*-}.0")
AZTAGS += env=$(PUBLISH_TARGET) commit=$(COMMIT)
GCPLABELS = env=$(PUBLISH_TARGET),commit=$(COMMIT)

ifeq ($(COMMIT),$(filter $(COMMIT), "", no-git))
  $(error "Bad value for COMMIT: '$(COMMIT)'")
endif

ifeq ($(AZVERSION),$(filter $(AZVERSION), "", ..0))
  $(error "Bad value for AZVERSION: '$(AZVERSION)')
endif

ifeq ($(ARCH),aarch64)
  AWSARCH = arm64
  AZARCH = arm64
  GCPARCH = arm64
else
  AWSARCH = x86_64
  AZARCH = x64
  GCPARCH = amd64
endif
ifeq ($(PUBLISH_TARGET),dev)
  AZGALLERY = vmtesting_dev
  AZRESOURCEGROUP = chainguard-vms
  GCPPROJECT = $(shell gcloud config get project)
  GCPBUCKET = $(GCPPROJECT)
  QEMU_GCSBUCKET = gs://$(GCPPROJECT)
  VMWARE_GCSBUCKET = gs://$(GCPPROJECT)
  RPI_GCSBUCKET = gs://$(GCPPROJECT)
else ifeq ($(PUBLISH_TARGET),staging)
  AZGALLERY = chainguard_vms_staging
  AZRESOURCEGROUP = chainguard-vms-staging
  GCPPROJECT = staging-vms-h8zx
  GCPBUCKET = wolfi-vm-images-workloads
  QEMU_GCSBUCKET = gs://wolfi-vm-images-workloads
  VMWARE_GCSBUCKET = gs://wolfi-vm-images-workloads
  RPI_GCSBUCKET = gs://wolfi-vm-images-workloads
else ifeq ($(PUBLISH_TARGET),eap)
# EAP is also considered "production" in that it hits any EAP end user right now.
  AZGALLERY = eap_chainguard_vms
  AZRESOURCEGROUP = chainguard-vms-prod
  GCPPROJECT = chainguard-vms-eap
  GCPBUCKET = wolfi-vm-images-workloads
  QEMU_GCSBUCKET = gs://chainguard-vms-eap
  VMWARE_GCSBUCKET = gs://chainguard-vms-eap
  RPI_GCSBUCKET = gs://wolfi-vm-images-workloads
else ifeq ($(PUBLISH_TARGET),production)
  AZGALLERY = chainguard_vms
  AZRESOURCEGROUP = chainguard-vms-prod
# Currently we are using wolfi-vm for internal images for workstations and other cases.
# TODO: Move production workstation images to the chainguard-workstations project.
  GCPPROJECT = wolfi-vm
  GCPBUCKET = wolfi-vm-images-workloads
  QEMU_GCSBUCKET = gs://wolfi-vm-images-workloads
  VMWARE_GCSBUCKET = gs://wolfi-vm-images-workloads
  RPI_GCSBUCKET = gs://wolfi-vm-images-workloads
else
  $(error "Bad value for PUBLISH_TARGET: '$(PUBLISH_TARGET)')
endif
QEMU_AZSTORAGEACCOUNT = chainguardvms$(PUBLISH_TARGET)
QEMU_AZSTORAGECONTAINER = chainguard-vms-qemu

awspub-%: AWSSTEM=$(subst awspub-aws-,,$@)
awspub-%: AWSNAME=$(PREFIX)-$(AWSSTEM)-$(AWSARCH)-$(BUILD_TIMESTAMP)
awspub-%: AWSSSM=$(PREFIX)-$(AWSSTEM)-$(AWSARCH)
awspub-%: $(ARCH_OUT_D)/%/disk.vmdk
	./tools/aws-image-upload --name=$(AWSNAME) --arch=$(AWSARCH) $(if $(SSM),--ssm=$(AWSSSM)) $(if $(SHARE),--share="$(SHARE)") $< $(BUCKET)

.PHONY: aws-create aws-create-% aws-publish aws-publish-%
# these are just so human can type 'make aws-create-aws-base' to do the create/publish
$(foreach name,$(disks_aws),aws-image-create-$(name)): aws-image-create-%: $(ARCH_OUT_D)/awspub/create/%.json
$(foreach name,$(disks_aws),aws-image-publish-$(name)): aws-image-publish-%: $(ARCH_OUT_D)/awspub/publish/%.output

aws-publish: $(foreach name,$(disks_aws),aws-publish-$(name))
aws-create: $(foreach name,$(disks_aws),aws-create-$(name))
aws-publish-aws-eks: $(foreach name,$(group_aws_eks),aws-image-publish-$(name))
aws-publish-aws-noneks: $(foreach name,$(group_aws_noneks),aws-image-publish-$(name))
aws-create-aws-eks: $(foreach name,$(group_aws_eks),aws-image-create-$(name))
aws-create-aws-noneks: $(foreach name,$(group_aws_noneks),aws-image-create-$(name))

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
$(ARCH_OUT_D)/azure-%/publish.$(PUBLISH_TARGET).json: AZTAGS+= local-name=azure-$*
$(ARCH_OUT_D)/azure-%/publish.$(PUBLISH_TARGET).json: $(ARCH_OUT_D)/azure-%/disk.raw
	@$(call capture_stdout,$@,\
		$(TOOLS_D)/azure-image-upload --arch=$(AZARCH) --gallery=$(AZGALLERY) \
		--name=$(AZNAME) --disk-name=$(AZNAME)-$(BUILD_TIMESTAMP) --image-version=$(AZVERSION) \
		--tags="$(AZTAGS)" --resource-group=$(AZRESOURCEGROUP) $(dir $@)disk.raw)

.PHONY: publish-gcp
publish-gcp: $(foreach name,$(disks_gcp),publish-gcp-$(subst gcp-,,$(name)))
$(foreach name,$(disks_gcp),publish-gcp-$(subst gcp-,,$(name))): publish-gcp-%: $(ARCH_OUT_D)/gcp-%/publish.$(PUBLISH_TARGET).yaml

$(ARCH_OUT_D)/gcp-%/publish.$(PUBLISH_TARGET).yaml: GCPNAME=$(PREFIX)-$*-$(GCPARCH)-$(BUILD_TIMESTAMP)
$(ARCH_OUT_D)/gcp-%/publish.$(PUBLISH_TARGET).yaml: GCPFAMILY=$(PREFIX)-$*-$(GCPARCH)
$(ARCH_OUT_D)/gcp-%/publish.$(PUBLISH_TARGET).yaml: export GCP_PROJECT = $(GCPPROJECT)
$(ARCH_OUT_D)/gcp-%/publish.$(PUBLISH_TARGET).yaml: $(ARCH_OUT_D)/gcp-%/disk.raw
	$(TOOLS_D)/google-image-upload --family="$(GCPFAMILY)" --name="$(GCPNAME)" \
	--arch="$(GCPARCH)" --labels="$(GCPLABELS),local-name=gcp-$*" "$(dir $@)disk.raw" "$(GCPBUCKET)"
	@$(call capture_stdout,$@, gcloud compute images describe --project "$(GCP_PROJECT)" "$(GCPNAME)")

.PHONY: publish-qemu
publish-qemu: $(foreach name,$(disks_qemu),publish-qemu-$(subst generic-,,$(name)))
$(foreach name,$(disks_qemu),publish-qemu-$(subst generic-,,$(name))): publish-qemu-%: $(ARCH_OUT_D)/generic-%/publish.$(PUBLISH_TARGET).json

$(ARCH_OUT_D)/generic-%/publish.$(PUBLISH_TARGET).json: $(ARCH_OUT_D)/generic-%/disk.raw $(ARCH_OUT_D)/generic-%/disk.qcow2
	@mkdir -p $(dir $@)
	$(call capture_stdout,$@, ./tools/generic-image-upload \
		--name generic-$* \
		--timestamp $(BUILD_TIMESTAMP) \
		--arch $(ARCH) \
		--gcs-bucket $(QEMU_GCSBUCKET) \
		--azure-account $(QEMU_AZSTORAGEACCOUNT) \
		--azure-container $(QEMU_AZSTORAGECONTAINER) \
		--raw-path $(dir $@)disk.raw \
		--qcow2-path $(dir $@)disk.qcow2 \
		--signed-urls)

.PHONY: publish-vmware
publish-vmware: $(foreach name,$(disks_vmware),publish-vmware-$(subst vmware-,,$(name)))
$(foreach name,$(disks_vmware),publish-vmware-$(subst vmware-,,$(name))): publish-vmware-%: $(ARCH_OUT_D)/vmware-%/publish.$(PUBLISH_TARGET).json

$(ARCH_OUT_D)/vmware-%/publish.$(PUBLISH_TARGET).json: $(ARCH_OUT_D)/vmware-%/disk.raw $(ARCH_OUT_D)/vmware-%/disk.vmdk $(ARCH_OUT_D)/vmware-%/disk-flat.vmdk
	@mkdir -p $(dir $@)
	$(call capture_stdout,$@, ./tools/generic-image-upload \
		--name vmware-$* \
		--timestamp $(BUILD_TIMESTAMP) \
		--arch $(ARCH) \
		--gcs-bucket $(VMWARE_GCSBUCKET) \
		--raw-path $(dir $@)disk.raw \
		--vmdk-path $(dir $@)disk.vmdk \
		--vmdk-flat-path $(dir $@)disk-flat.vmdk)

.PHONY: publish-rpi
publish-rpi: $(foreach name,$(disks_rpi),publish-rpi-$(subst rpi-generic-,,$(name)))
$(foreach name,$(disks_rpi),publish-rpi-$(subst rpi-generic-,,$(name))): publish-rpi-%: $(ARCH_OUT_D)/rpi-generic-%/publish.$(PUBLISH_TARGET).json

$(ARCH_OUT_D)/rpi-generic-%/publish.$(PUBLISH_TARGET).json: $(ARCH_OUT_D)/rpi-generic-%/disk.raw
	@mkdir -p $(dir $@)
	$(call capture_stdout,$@, ./tools/generic-image-upload \
		--name rpi-generic-$* \
		--timestamp $(BUILD_TIMESTAMP) \
		--arch $(ARCH) \
		--gcs-bucket $(RPI_GCSBUCKET) \
		--raw-path $(dir $@)disk.raw \
		--signed-urls)

# we use the stdout of awspub publish to indicate the thing was published.
$(ARCH_OUT_D)/awspub/publish/%.output: output/awspub.mapping $(ARCH_OUT_D)/awspub/create/%.json
	@$(call capture_stdout,$@,\
	awspub publish --config-mapping=output/awspub.mapping awspub/$(ARCH)/$*.yaml)

$(ARCH_OUT_D)/%/disk.raw: configs/%/build.yaml apkoaas $(BUILDER_KERNEL) $(BUILDER_INITRD)
	@mkdir -p $(dir $@)
	$(TOP_D)/apkoaas build \
	--log-level=debug \
	--arch=$(ARCH) \
	--build-arch=$(BUILDER_ARCH) \
	--builder-cpio=$(BUILDER_INITRD) \
	--kernel=$(BUILDER_KERNEL) \
	--output=$@ \
	configs/$*/build.yaml

$(ARCH_OUT_D)/%/disk-debug.raw: apkoaas $(BUILDER_KERNEL) $(BUILDER_DEBUG_INITRD)
	@mkdir -p $(dir $@)
	$(TOP_D)/apkoaas build \
	--log-level=debug \
	--arch=$(ARCH) \
	--build-arch=$(BUILDER_ARCH) \
	--builder-cpio=$(BUILDER_DEBUG_INITRD) \
	--kernel=$(BUILDER_KERNEL) \
	--output=$@ \
	configs/$*/build.yaml "$@"

.PHONY: clean
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

.PRECIOUS: $(foreach bname,disk.raw disk-debug.raw disk.vmdk disk-flat.vmdk disk.qcow2 disk.vhd image.tar initramfs.cpio,$(ARCH_OUT_D)/%/$(bname))
.PRECIOUS: configs/%/build.yaml builder/ovmf-%.fd

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

.PHONY: azure-marketplace-install-extension
azure-marketplace-install-extension:
# I don't particularly want to clone this from my repo during every image build
# and build the wheel particularly since you need the azdev tooling as well.
#
# Might be better if it was a chainguard repo?
#
# For now I'll just include the wheel if someone in review has a preferred method
# I'm happy to adopt that.
#
# Here are instructions for how to generate the wheel and install the extension
#
# az is convenient because it doesn't require us to add additional creds
# to github secrets and instead can use the az auth stack.
#
# This should work to rebuild it.
# TMPDIR := $(shell mktemp -d)
# REVISION := 11f470f205c89c0d6671869c70dd7a4475a09778
# git clone --revision $(REVISION) https://github.com/justinvreeland/partnercenter-cli-extension.git \
# $(TMPDIR)/partnercenter-cli-extension
# $(cd partnercenter && azdev extension build --verbose partnercenter)
#
# Fails if the extension wasn't installed to begin with which it shouldn't be
	az extension remove -n partnercenter || true
	az extension add --yes --source whl/partnercenter-0.2.7-py3-none-any.whl

.PHONY: azure-marketplace-update-technical-plan
azure-marketplace-update-technical-plan: $(foreach name,$(disks_azure),azure-marketplace-update-technical-plan-$(subst azure-,,$(name)))
$(foreach name,$(disks_azure),azure-marketplace-update-technical-plan-$(subst azure-,,$(name))): azure-marketplace-update-technical-plan-%: output/azure-marketplace-update-technical-plan-%s.json

output/azure-marketplace-update-technical-plan-%s.json: AZNAME=$(PREFIX)-$*
output/azure-marketplace-update-technical-plan-%s.json: azure-marketplace-install-extension
# We don't want to publish devel or testing images by mistake
ifneq ("$(AZGALLERY)","eap_chainguard_vms")
	$(error This gallery is not allowed listed in the makefile if you want to upload images from it please update the makefile.)
endif
ifneq ("$(AZRESOURCEGROUP)","chainguard-vms-prod")
	$(error This resouce group is not allowed listed in the makefile if you want to upload images from it please update the makefile.)
endif
	$(call capture_stdout,$@,\
		$(TOOLS_D)/azure-marketplace-add-vm-image-version \
		--image-version=$(AZVERSION) --image-name=$(AZNAME) \
		--gallery=$(AZGALLERY) --plan=$(AZMARKETPLACE_PLAN) \
		--offer=$(AZMARKETPLACE_OFFER) --resource-group=$(AZRESOURCEGROUP) )
	@result=$$(jq -rs '.[0].result' "$@"); \
	if [ "$$result" = "failed" ]; then exit 1; fi; \
	job_result=$$(jq -rs '.[1].job_result' "$@"); \
	if [ "$$job_result" = "failed" ]; then exit 1; fi

# Yam doesn't recurse
.PHONY: lint-configs
lint-configs:
	find ./ -name '*.yaml' -regex './configs.*' -exec yam {} \;
