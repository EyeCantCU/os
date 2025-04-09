runners=$(subst pkg/runner/,,$(wildcard pkg/runner/*))
go_files=$(shell find pkg -name *.go)

all: runners
runner_targets=$(foreach runner,$(runners),runner/$(runner))
runners: $(runner_targets)
$(runner_targets): runner/%: $(go_files)
	@mkdir -p runner
	go build -C ./pkg/runner/$* -o ../../../runner/$*

.PHONY: clean
clean:
	rm -rf runner/*
