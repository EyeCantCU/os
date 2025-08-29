# This Makefile mostly exists as an affordance for testing.
# We probably want to replace it with something else.
# Primarily, this just multiplexes commands across each subdirectory.
pkgs := $(shell stereo make targets)

pkg_targets = $(foreach name,$(pkgs),package/$(name))
$(pkg_targets): package/%:
	stereo make package $*

test_targets = $(foreach name,$(pkgs),test/$(name))
$(test_targets): test/%:
	stereo make test $*

debug_targets = $(foreach name,$(pkgs),debug/$(name))
$(debug_targets): debug/%:
	stereo make debug $*

test_debug_targets = $(foreach name,$(pkgs),test-debug/$(name))
$(test_debug_targets): test-debug/%:
	stereo make test-debug $*

.PHONY: clean
clean:
	make -C os clean
	make -C extra-packages clean
	make -C enterprise-packages clean
