---
name: build-test
description: Build and test melange packages. Use after creating or modifying package YAML files to verify they work correctly.
---

# Build and Test Packages

Use this skill after creating or modifying package YAML files to build and test them.

## Prerequisites

Before building, ensure you have:
1. `melange` installed and in PATH
2. Authenticated with chainctl: `make apk-token`

## Build Commands

### Build a package
```bash
make package/NAME
```
Example: `make package/perl-timedate`

### Test a package
```bash
make test/NAME
```
Example: `make test/perl-timedate`

### Debug a failing build (interactive)
```bash
make debug/NAME
```
Drops into an interactive shell on failure for debugging.

### Debug a failing test (interactive)
```bash
make test-debug/NAME
```
Drops into an interactive shell on test failure.

## Build Order

**Always build dependencies first.** If package A depends on package B:
1. `make package/B` - build dependency
2. `make test/B` - verify dependency works
3. `make package/A` - build main package
4. `make test/A` - verify main package works

## Common Issues

### "package not found" during build
The dependency hasn't been built yet. Build dependencies first.

### Test fails to find package
The package hasn't been built, or was built for a different architecture. Run `make package/NAME` first.

### Authentication errors
Run `make apk-token` to refresh authentication.

### Build hangs or fails mysteriously
Try `make debug/NAME` to get an interactive shell and investigate.

## Architecture

By default, builds for the current architecture. Override with:
```bash
ARCH=x86_64 make package/NAME
ARCH=aarch64 make package/NAME
```

## Output Location

Built packages go to: `packages/${ARCH}/`

## Workflow Checklist

After creating a new package:
- [ ] Build dependencies first (if any)
- [ ] `make package/NAME` succeeds
- [ ] `make test/NAME` passes
- [ ] Check output for warnings or unexpected behavior
