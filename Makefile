TOP_D := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
TOOLS_D = $(TOP_D)/tools
# when converting from an existing image, we stuff these in.
BOOT_PKGS ?= linux-boot-configuration mattmoor-chainit-init
ALL_DISKS := generic google docker-runner workstation
SIZE ?= auto

ARCH ?= $(shell uname -m)
ifeq ($(ARCH), arm64)
	ARCH = aarch64
else ifeq ($(ARCH), amd64)
	ARCH = x86_64
endif

BUILDER_ARCH ?= $(shell uname -m)
BUILDER_KERNEL = builder/kernel-$(BUILDER_ARCH)
BUILDER_INITRD = builder/initrd-$(BUILDER_ARCH)
BUILDER_DEPS = $(BUILDER_KERNEL) $(BUILDER_INITRD)

# this needs fixing if running non-native qemu-system
OVMF_FIRMWARE = builder/ovmf-$(ARCH).fd

QEMU_CMD := none
ifeq (${ARCH}, aarch64)
	QEMU_CMD = qemu-system-${ARCH} -cpu max -machine virt -accel hvf
else ifeq (${ARCH}, x86_64)
	QEMU_CMD = qemu-system-${ARCH} -cpu max -machine q35 -accel tcg
endif

ifeq ($(AUTH_TOK),)
AUTH_TOK := $(shell chainctl auth token --audience apk.cgr.dev || echo bad-token)
ifeq ($(AUTH_TOK),$(filter $(AUTH_TOK), "", bad-token))
$(error "Failed to get an auth token")
endif
endif

# set to 'vnc:1' to run a vnc server that you can connect to
QEMU_DISPLAY ?= none
# Append the several common arguments to the QEMU_CMD
QEMU_CMD += -m 3G -display $(QEMU_DISPLAY) -serial mon:stdio -echr 0x05 -device virtio-rng-pci

INITRD_CMD := ${QEMU_CMD}
ifeq (${ARCH}, aarch64)
	CONSOLE_QUIET = quiet
else ifeq (${ARCH}, x86_64)
	CONSOLE_QUIET = console=ttyS0 quiet
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
 "--size=$(3)" $(4) "$(1)" "$(2)"

.PHONY: disks
disks: $(foreach name,$(ALL_DISKS),output/$(name)/disk.raw)

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

output/%/image.tar: configs/%.yaml
	@mkdir -p $(dir $@)
	@$(call apko_build,minirootfs,$<,$@.gz.tmp,)
	t=$@.tmp$$$$; gunzip --to-stdout "$@.gz.tmp" > "$$t" && \
		mv "$$t" "$@" || { rm -f "$$t"; exit 1; }
	rm $@.gz.tmp

output/%/initrd.cpio: configs/%.yaml
	@mkdir -p $(dir $@)
	@$(call apko_build,cpio,$<,$@,,)

output/%/disk-debug.raw: $(BUILDER_DEPS) output/%/image.tar $(TOOLS_D)/install-target-disk
	$(call tar2efi,output/$*/image.tar,$@,$(SIZE),--env=DEBUG=true)

output/%/disk.raw: $(BUILDER_DEPS) output/%/image.tar $(TOOLS_D)/install-target-disk
	$(call tar2efi,output/$*/image.tar,$@,$(SIZE))

run-builder-%: $(BUILDER_DEPS) output/%/image.tar
	$(call tar2efi,output/$*/image.tar,$@,$(SIZE),--env=DEBUG=true --workload=$(TOOLS_D)/debug-shell)

output/%/disk.tar.gz: output/%/disk.raw
	$(TOOLS_D)/google-image-upload create-image-tgz output/$*/disk.raw $@

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

run-initrd-%: output/%/initrd.cpio $(KERNEL)
	@$(call boot_initrd,$(KERNEL),output/$*/initrd.cpio,) $(QEMU_NETFLAGS)

# second serial console (ttyS1) will get a systemd.debug-shell
# connect to it with: socat STDIO,cfmakeraw,isig=1 UNIX:output/generic/.socket.ttyS1
debug-disk-%: output/%/disk-debug.raw $(FIRMWARE)
	$(call boot_disk,$<) $(QEMU_NETFLAGS) -serial unix:$(dir $<)/.socket.ttyS1,wait=off,server=on -snapshot

run-disk-%: output/%/disk.raw $(FIRMWARE)
	$(call boot_disk,$<) $(QEMU_NETFLAGS)

shell-initrd: run-initrd-chainguard-base
shell-disk: run-disk-chainguard-base

clean:
	rm -Rf output builder

.PRECIOUS: $(foreach bname,disk.raw disk-debug.raw image.tar initrd.cpio,output/%/$(bname))
.PRECIOUS: configs/%.yaml
