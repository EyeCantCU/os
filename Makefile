runners=$(subst pkg/runner/,,$(wildcard pkg/runner/*))

runner_targets=$(foreach runner,$(runners),runner/$(runner))
.PHONY: $(runner_targets)
$(runner_targets): runner/%:
	@mkdir -p runner
	go build -C ./pkg/runner/$* -o ../../../runner/$*

.PHONY: clean
clean:
	rm -rf runner/*
