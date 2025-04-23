MAKEFLAGS := --jobs=$(shell command -v nproc >/dev/null && nproc || echo 4)
MAKEFLAGS += --output-sync=target

TEST_ARCHES := arm64 amd64
runners=$(subst pkg/runner/,,$(wildcard pkg/runner/*))
tests=$(subst pkg/test/,,$(wildcard pkg/test/*))

go_files=$(shell find pkg -name *.go -and -not -name '*_generated.go')
go_gen_sources = $(git grep -l '^//go:generate')

go_tools=$(shell go list -tags tools -f '{{join .Imports " "}}' -e ./pkg/tools/)
go_tools_bin=$(foreach tool,$(notdir $(go_tools)),tools/$(tool))

all: runners tests

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

test_targets=$(foreach arch,$(TEST_ARCHES),$(foreach test,$(tests),test/$(arch)/$(test)))
tests: $(test_targets)

# getarch(prefix,string) returns the first path token in string after removing prefix.
#   getarch(test/,test/myarch/bob) -> myarch
getarch = $(firstword $(subst /, ,$(subst $(1),,$(2))))

# test_targets are test/<arch>/<name>
#  where <name> is a dir in pkg/test
$(test_targets): test/%: $(go_files) .go-generated
	@mkdir -p $(notdir $@)
	GOARCH=$(call getarch,test/,$@) go test -c -o $@ ./pkg/test/$(notdir $@) -tags vmtest

gofmt: .go-formatted
.go-formatted: $(go_files)
	o=$$(gofmt -l -w pkg/ 2>&1) && [ -z "$$o" ] || \
		{ echo "gofmt made changes: $$o" 1>&2; exit 1; }
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
