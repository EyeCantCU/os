MAKEFLAGS := --jobs=$(shell command -v nproc >/dev/null && nproc || echo 4)
MAKEFLAGS += --output-sync=target
HASH := \#

TEST_ARCHES := x86_64 aarch64
runners=$(subst pkg/runner/,,$(wildcard pkg/runner/*))
# grab 'artifacts' from vms-test/pkg/artifacts/cmd/root.go
cmds=$(subst /cmd,,$(subst pkg/,,$(wildcard pkg/*/cmd)))

test_groups=$(subst pkg/test/,,$(wildcard pkg/test/*))
tests=$(foreach group,$(test_groups),$(subst pkg/test/,,$(wildcard pkg/test/$(group)/*)))

go_files=$(shell find pkg -name '*.go' -and -not -name '*_generated.go')
go_gen_sources = $(git grep -l '^//go:generate')

go_tools=$(shell go list -tags tools -f '{{join .Imports " "}}' -e ./pkg/tools/)
go_tools_bin=$(foreach tool,$(notdir $(go_tools)),tools/$(tool))

shell_scripts:=$(shell git grep -lIE "^$(HASH)!/(usr/)?s?bin/(env )?(ba)?sh")

all: runners tests cmds

$(go_tools_bin): go.mod pkg/tools/tools.go
	@mkdir -p tools/
	@TOOL_PKG=$(filter %/$(@F),${go_tools}); \
	TOOL=$$TOOL_PKG; \
	TOOL_MODULE=""; \
	while true; do \
		# Check if the current package has a valid module version, if so we're done \
		TOOL_MODULE=$$(go list -f "{{if eq .Path \"$$TOOL\"}}{{.Path}}@{{.Version}}{{end}}" -m all); \
		[ -n "$$TOOL_MODULE" ] && break; \
		# Chop off the last bit of the URL, if we're down to the last part give up \
		# Otherwise continue with the trimmed URL \
		NEW=$${TOOL%/*}; \
		[ "$$TOOL" = "$$NEW" ] && break; \
		TOOL=$$NEW; \
	done; \
	echo "Resolved tool package $$TOOL_PKG to module $$TOOL_MODULE"; \
	# Print the command manually since we silenced this long one-liner \
	echo "GOBIN=\$$(pwd)/tools/ go install $${TOOL_PKG}@$${TOOL_MODULE##*@}"; \
	GOBIN=$$(pwd)/tools/ go install $${TOOL_PKG}@$${TOOL_MODULE##*@}

.go-generated: $(go_gen_sources) $(go_tools_bin)
	PATH=$$PWD/tools/:$$PATH go generate ./...
	touch $@

runner_targets=$(foreach runner,$(runners),runner/$(runner))
runners: $(runner_targets)
$(runner_targets): runner/%: $(go_files) .go-generated
	@mkdir -p runner
	go build -C ./pkg/runner/$* -o ../../../runner/$*

cmd_targets=$(foreach cmd,$(cmds),tools/$(cmd))
cmds: $(cmd_targets)
$(cmd_targets): tools/%: $(go_files) .go-generated
	@mkdir -p tools
	go build -C ./pkg/$*/cmd -o ../../../tools/$*

test_targets=$(foreach arch,$(TEST_ARCHES),$(foreach test,$(tests),test/$(arch)/$(test)))
tests: $(test_targets)

# getarch(prefix,string) returns the first path token in string after removing prefix.
#   getarch(test/,test/myarch/bob) -> myarch
getarch = $(firstword $(subst /, ,$(subst $(1),,$(2))))
# goarch converts x86_64 to arm64 and aarch64->arm64
goarch = $(subst aarch64,arm64,$(subst x86_64,amd64,$(call getarch,$(1),$(2))))

# test_targets are test/<arch>/<group/<name>
#  where <group> is a dir in pkg/test
#  and <name> is a dir in pkg/test/<group>
$(test_targets): test/%: $(go_files) .go-generated
	@mkdir -p $(dir $@)
	GOOS=linux GOARCH=$(call goarch,test/,$@) go test -c -o $@ ./pkg/$(subst $(call getarch,test/,$@)/,,$(@)) -tags vmtest

gofmt: .go-formatted
.go-formatted: $(go_files)
	o=$$(gofmt -l -w pkg/ 2>&1) && [ -z "$$o" ] || \
		{ echo "gofmt made changes: $$o" 1>&2; exit 1; }
	@touch $@

shellcheck: .shellcheck
.shellcheck: $(shell_scripts)
	@rc=0;for script in $(shell_scripts); do \
	    echo "shellcheck $$script"; \
	    shellcheck "$$script" || rc=$$?; \
	done; exit $$rc
	@touch $@

.PHONY: unit-test
unit-test: $(go_files) .go-generated
	go test ./... -tags unittest

.PHONY: clean
clean:
	rm -rf tools/*
	rm -rf test/*
	rm -rf runner/*
	rm -f .go-generated
