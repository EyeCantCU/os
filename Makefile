runners=$(subst pkg/runner/,,$(wildcard pkg/runner/*))
tests=$(subst pkg/test/,,$(wildcard pkg/test/*))

go_files=$(shell find pkg -name *.go -and -not -name '*_generated.go')
generated_go_files=$(shell find pkg -name *_generated.go)

go_tools=$(shell go list -tags tools -f '{{join .Imports " "}}' -e ./pkg/tools/)
go_tools_bin=$(foreach tool,$(notdir $(go_tools)),tools/$(tool))

$(go_tools_bin): go.mod pkg/tools/tools.go
	@mkdir -p tools/
	go build -v -o $@ $(filter %/$(@F),${go_tools})

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
