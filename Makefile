runners=$(subst pkg/runner/,,$(wildcard pkg/runner/*))
tests=$(subst pkg/test/,,$(wildcard pkg/test/*))

go_files=$(shell find pkg -name *.go -and -not -name '*_generated.go')
generated_go_files=$(shell find pkg -name *_generated.go)

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

.PHONY: go-generate
go-generate: $(generated_go_files)
$(generated_go_files): $(go_files) $(go_tools_bin)
	PATH=$$(pwd)/tools/:$$PATH go generate ./...

all: runners tests
runner_targets=$(foreach runner,$(runners),runner/$(runner))
runners: $(runner_targets)
$(runner_targets): runner/%: $(go_files) $(generated_go_files)
	@mkdir -p runner
	go build -C ./pkg/runner/$* -o ../../../runner/$*

test_targets=$(foreach test,$(tests),test/$(test))
tests: $(test_targets)
$(test_targets): test/%: $(go_files) $(generated_go_files)
	@mkdir -p test
	go test -C ./pkg/test/$* -c -o ../../../test/$* -tags vmtest

.PHONY: unit-test
unit-test: $(go_files) $(generated_go_files)
	go test ./... -tags unittest

.PHONY: clean
clean:
	rm -rf tools/*
	rm -rf test/*
	rm -rf runner/*
