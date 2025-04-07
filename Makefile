runners=$(subst pkg/,,$(wildcard pkg/*))

runner_targets=$(foreach runner,$(runners),runner/$(runner))
.PHONY: $(runner_targets)
$(runner_targets): runner/%:
	@mkdir -p runner
	go build -C ./pkg/$* -o ../../runner/$*

.PHONY: clean
clean:
	rm -rf runner/*
