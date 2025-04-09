runners=$(subst pkg/runner/,,$(wildcard pkg/runner/*))
tests=$(subst pkg/test/,,$(wildcard pkg/test/*))
go_files=$(shell find pkg -name *.go)

all: runners tests
runner_targets=$(foreach runner,$(runners),runner/$(runner))
runners: $(runner_targets)
$(runner_targets): runner/%: $(go_files)
	@mkdir -p runner
	go build -C ./pkg/runner/$* -o ../../../runner/$*

test_targets=$(foreach test,$(tests),test/$(test))
tests: $(test_targets)
$(test_targets): test/%: $(go_files)
	@mkdir -p test
	go test -C ./pkg/test/$* -c -o ../../../test/$* -tags vmtest

.PHONY: unit-test
unit-test:
	go test ./... -tags unittest

.PHONY: clean
clean:
	rm -rf test/*
	rm -rf runner/*
