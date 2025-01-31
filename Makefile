TOP_D := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
TOOLS_D = $(TOP_D)/tools
# when converting from an existing image, we stuff these in.
BOOT_PKGS ?= linux-boot-configuration mattmoor-chainit-init
ALL_DISKS := generic google docker-runner workstation
SIZE ?= auto

ifeq ($(AUTH_TOK),)
AUTH_TOK := $(shell chainctl auth token --audience apk.cgr.dev || echo bad-token)
ifeq ($(AUTH_TOK),$(filter $(AUTH_TOK), "", bad-token))
$(error "Failed to get an auth token")
endif
endif

ARCHES := aarch64 x86_64
# Darwin reports arm64 for 'uname -m'
BUILDER_ARCH := $(shell uname -m)
# Darwin reports arm64 for 'uname -m'
ifeq ($(BUILDER_ARCH),arm64)
BUILDER_ARCH=aarch64
endif
ARCH := $(BUILDER_ARCH)

ifneq ($(filter-out $(ARCHES),$(BUILDER_ARCH)),)
$(error BUILDER_ARCH '$(BUILDER_ARCH)' not supported. Must be one of $(ARCHES))
endif
BUILDER_KERNEL = builder/kernel-$(BUILDER_ARCH)
BUILDER_INITRD = builder/initrd-$(BUILDER_ARCH)
BUILDER_DEPS = $(BUILDER_KERNEL) $(BUILDER_INITRD)

ifneq ($(filter-out $(ARCHES),$(ARCH)),)
$(error ARCH $(ARCH) not supported. Must be one of $(ARCHES))
endif

ARCH_OUT_D = output/$(ARCH)

# this needs fixing if running non-native qemu-system
OVMF_FIRMWARE = builder/ovmf-$(ARCH).fd

QEMU_MFLAGS ?= $(shell $(TOOLS_D)/qemu-machine-args $(ARCH))
QEMU_CMD = qemu-system-$(ARCH) $(QEMU_MFLAGS)

# set to 'vnc:1' to run a vnc server that you can connect to
QEMU_DISPLAY ?= none
# Append the several common arguments to the QEMU_CMD
QEMU_CMD += -m 3G -display $(QEMU_DISPLAY) -serial mon:stdio -echr 0x05 -device virtio-rng-pci

INITRD_CMD := $(QEMU_CMD)

ifeq ($(ARCH), aarch64)
CONSOLE_QUIET = quiet
# https://unix.stackexchange.com/questions/479085/
add-serial-debug = \
 -chardev socket,path=$(1),server=on,wait=off,id=debugshell \
 -device pci-serial,id=serial0,chardev=debugshell
else ifeq ($(ARCH), x86_64)
CONSOLE_QUIET = console=ttyS0 quiet
add-serial-debug = -serial unix:$(1),wait=off,server=on
endif

QEMU_NETFLAGS ?= -device virtio-net-pci,netdev=id1 \
 -netdev user,id=id1,hostfwd=tcp:127.0.0.1:6379-:6379

define qemu-initrd
	$(INITRD_CMD) \
	  -kernel $1 \
	  -initrd $2
endef

# qemubuild(workload-dir,script,outdir)
# map workload-dir and outdir into vm, and execute script
# see mkvm.yaml's script for reading the cmdline
plan9 = -device "virtio-9p-pci,id=fs$(1),fsdev=fsdev$(1),mount_tag=$(2)" \
 -fsdev "local,security_model=mapped,id=fsdev$(1),path=$(3)"

boot_initrd = $(call qemu-initrd,$1,$2) -append "$(CONSOLE_QUIET) $3"
boot_disk = $(call qemu-disk,$1)

raw_disk_args = \
 -blockdev driver=raw,node-name=$(notdir $1),file.driver=file,file.filename=$(1) \
 -device virtio-blk-pci,drive=$(notdir $1),serial=$(2),discard=true


define qemu-disk
	$(QEMU_CMD) \
	  -drive "if=pflash,format=raw,file=builder/ovmf-$(ARCH).fd,readonly=on" \
	  $(call raw_disk_args,$1,boot-disk)
endef

# calling 'withauth' like this: $(call withauth,cmd arg1 arg2...)'
# will print 'cmd arg1 arg2' and put HTTP_AUTH with AUTH_TOK into the environment.
withauth = echo $1 && env HTTP_AUTH="basic:apk.cgr.dev:user:$(AUTH_TOK)" $1
apko_build = $(call withauth,apko build-$(1) \
 --build-repository-append="https://apk.cgr.dev/chainguard-private" \
 $(call add_apk_repos,$(EXTRA_REPO_DIRS)) $(5) \
 --package-append="$4" $(2) $(3))

add_apk_repos = $(foreach dir,$1,\
 --build-repository-append=$(dir)/packages --keyring-append=$(wildcard $(dir)/local-*.pub))

tar2efi = $(TOOLS_D)/tar2efi-disk \
 "--kernel=$(BUILDER_KERNEL)" "--initrd=$(BUILDER_INITRD)" \
 "--workload=$(TOOLS_D)/install-target-disk" \
 "--boot-arch=$(BUILDER_ARCH)" \
 "--env=ARCH=$(ARCH)" \
 "--size=$(3)" $(4) "$(1)" "$(2)"

.PHONY: disks
disks: $(foreach name,$(ALL_DISKS),$(ARCH_OUT_D)/$(name)/disk.raw)

cfgs = $(wildcard configs/*.yaml)
disks = $(foreach cfg,$(cfgs),$(subst .yaml,,$(notdir $(cfg))))

disk_targets = $(foreach name,$(disks),disk-$(name))
.PHONY: $(disk_targets)
$(disk_targets): disk-%: $(ARCH_OUT_D)/%/disk.raw

check:
	@echo "This does not do anything useful, but it passes. Please improve."

configs/%.yaml:
	@mkdir -p $(dir $@)
	cosign verify-attestation \
		--type=https://apko.dev/image-configuration \
		--certificate-oidc-issuer=https://token.actions.githubusercontent.com \
		--certificate-identity=https://github.com/chainguard-images/images-private/.github/workflows/release.yaml@refs/heads/main \
		"cgr.dev/chainguard-private/$*" > $@.tmp
	$(TOOLS_D)/config-insert-packages $< $@.tmp $(BOOT_PKGS)
	yq -P -i '.payload | @base64d | fromjson | .predicate' $@.tmp
	mv $@.tmp $@

$(ARCH_OUT_D)/%/image.tar: configs/%.yaml
	@mkdir -p $(dir $@)
	@$(call apko_build,minirootfs,$<,$@.gz.tmp,,--build-arch=$(ARCH))
	t=$@.tmp$$$$; gunzip --to-stdout "$@.gz.tmp" > "$$t" && \
		mv "$$t" "$@" || { rm -f "$$t"; exit 1; }
	rm $@.gz.tmp

$(ARCH_OUT_D)/%/initrd.cpio: configs/%.yaml
	@mkdir -p $(dir $@)
	@$(call apko_build,cpio,$<,$@,,)

$(ARCH_OUT_D)/%/disk-debug.raw: $(BUILDER_DEPS) $(ARCH_OUT_D)/%/image.tar $(TOOLS_D)/install-target-disk
	$(call tar2efi,$(ARCH_OUT_D)/$*/image.tar,$@,$(SIZE),--env=DEBUG=true)

$(ARCH_OUT_D)/%/disk.raw: $(BUILDER_DEPS) $(ARCH_OUT_D)/%/image.tar $(TOOLS_D)/install-target-disk
	$(call tar2efi,$(ARCH_OUT_D)/$*/image.tar,$@,$(SIZE))

run-builder-%: $(BUILDER_DEPS) $(ARCH_OUT_D)/%/image.tar
	$(call tar2efi,$(ARCH_OUT_D)/$*/image.tar,$@,$(SIZE),--env=DEBUG=true --workload=$(TOOLS_D)/debug-shell)

$(ARCH_OUT_D)/%/disk.tar.gz: $(ARCH_OUT_D)/%/disk.raw
	$(TOOLS_D)/google-image-upload create-image-tgz $(ARCH_OUT_D)/$*/disk.raw $@

.PHONY: builder
builder: $(BUILDER_DEPS)

builder/kernel-%: $(TOOLS_D)/grab-pkg-artifact
	@mkdir -p $(dir $@)
	@$(call withauth,$(TOOLS_D)/grab-pkg-artifact "--arch=$*" kernel $@)

builder/initrd-%: mkvm.yaml
	@mkdir -p $(dir $@)
	@$(call apko_build,cpio,$<,$@,,--build-arch=$*)

builder/ovmf-%.fd: $(TOOLS_D)/grab-pkg-artifact
	@mkdir -p $(dir $@)
	$(TOOLS_D)/grab-pkg-artifact "--arch=$*" ovmf $@

%.vmdk: %.raw
	qemu-img convert -O vmdk -o subformat=streamOptimized $< $@

run-initrd-%: $(ARCH_OUT_D)/%/initrd.cpio $(KERNEL)
	@$(call boot_initrd,$(KERNEL),$(ARCH_OUT_D)/$*/initrd.cpio,) $(QEMU_NETFLAGS)

# second serial console (ttyS1) will get a systemd.debug-shell
# connect to it with: socat STDIO,cfmakeraw,isig=1 UNIX:$(ARCH_OUT_D)/generic/.socket.debug-shell
debug-disk-%: $(ARCH_OUT_D)/%/disk-debug.raw $(OVMF_FIRMWARE)
	$(call boot_disk,$<) $(QEMU_NETFLAGS) $(call add-serial-debug,$(dir $<)/.socket.debug-shell) -snapshot

.PHONY: debug-shell-%
debug-shell-%:
	@echo "::: Make sure you have a 'debug-disk-$*' session running or this wont work"
	@echo "[hit enter]"
	@socat STDIO,cfmakeraw,isig=1 UNIX:$(ARCH_OUT_D)/$*/.socket.debug-shell

run-disk-%: $(ARCH_OUT_D)/%/disk.raw $(OVMF_FIRMWARE)
	$(call boot_disk,$<) $(QEMU_NETFLAGS)

shell-initrd: run-initrd-chainguard-base
shell-disk: run-disk-chainguard-base

clean:
	rm -Rf output builder

.PRECIOUS: $(foreach bname,disk.raw disk-debug.raw image.tar initrd.cpio,$(ARCH_OUT_D)/%/$(bname))
.PRECIOUS: configs/%.yaml builder/ovmf-%.fd
