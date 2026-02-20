---
name: add-package-tests
description: Add or improve top-level test stanzas to melange package YAML files. Use when a package is missing a top-level `test:` block, has only a placeholder test, or needs better functional coverage.
---

# Add Package Tests Skill

Follow CLAUDE.md "Package Test Best Practices" section throughout. This skill covers the workflow for investigating a package and writing good tests for it.

## 1. Understand the Package

Before writing any tests, read the YAML file and understand what the package actually installs.

```bash
# Read the package YAML
cat os/PACKAGE.yaml
```

Key questions to answer:
- Does it install **binaries** in `/usr/bin`? (→ `ldd-check`, `help-check`, `ver-check`, functional tests)
- Does it install **shared libraries** (`.so`)? (→ `ldd-check`)
- Is it a **meta/virtual package** with only runtime deps and no files? (→ `metapackage` or `byproductpackage`)
- Is it a **compat/symlink package**? (→ `symlink-check`)
- Is it a **daemon or service**? (→ `daemon-check-output`)
- Does it install a **Python module**? (→ `python/import`)
- Is it a **Ruby gem**? (→ `gem-check`)
- Is it a **byproduct** of another build (lower provider-priority)? (→ `byproductpackage`)

Also check what **subpackages** exist and whether they already have tests.

## 2. Choose the Right Test Type

### Decision Table

| Package characteristics | Recommended top-level test |
|---|---|
| Installs ELF binaries or `.so` libraries | `test/tw/ldd-check` + functional tests |
| CLI tool with `--help` / `--version` | `test/tw/help-check` and/or `test/tw/ver-check` |
| Daemon / long-running service | `test/daemon-check-output` |
| Meta package (only `dependencies.runtime`, no files) | `test/metapackage` |
| Virtual package (provides another package name) | `test/virtualpackage` |
| Byproduct of another build (lower provider-priority, superseded by a higher-priority provider) | `test/tw/byproductpackage` |
| Intentionally empty (only `spdx.json`) | `test/tw/emptypackage` |
| Compat / symlink-only package | `test/tw/symlink-check` |
| Python package | `python/import` |
| Ruby gem | `test/tw/gem-check` |
| Systemd service files | `test/tw/verify-service` |

### Subpackage-Specific Tests

| Subpackage type | Recommended test |
|---|---|
| `-dev` (headers, `.pc` files) | `test/pkgconf` (if has `.pc`) + `test/tw/ldd-check` |
| `-dev` (C/C++ headers) | `test/tw/header-check` + `test/tw/devpackage` |
| `-libs` (shared libs) | `test/tw/ldd-check` |
| `-static` | `test/tw/staticpackage` |
| `-doc` | `test/tw/docs` |
| `-compat` | `test/tw/symlink-check` |
| `-debug` | `test/tw/debugpackage` |

## 3. Write Functional Tests

**Tests must validate real functionality — not just `--help` or `--version`.**

Config syntax validation (`tool -t config`) is NOT a functional test. Functional tests must exercise the binary doing its actual job.

### Shell Script Rules (from CLAUDE.md)

```bash
# Only use set -euo pipefail when the test block contains unix pipes
# (pipefail is only meaningful when pipes are present)
set -euo pipefail
some_command | grep -F "expected string"

# For test blocks with no pipes, omit set -euo pipefail entirely
tool-name --version
tool-name validate --file /etc/tool/config.yaml

# Use grep -F for literal strings, never grep -q
some_command | grep -F "expected string"

# Use jq -e for JSON — not grep/head/tail
curl -sf http://localhost:8080/api | jq -e '.status == "ok"'
```

### CLI Tool Test Template

```yaml
test:
  pipeline:
    - uses: test/tw/ldd-check
    - uses: test/tw/ver-check
      with:
        bins: tool-name
        version: ${{package.version}}
    - runs: |
        # No pipes here — omit set -euo pipefail
        tool-name validate --file /etc/tool/config.yaml
    - runs: |
        set -euo pipefail
        # Pipes present — use set -euo pipefail
        echo "test input" | tool-name process
        tool-name list | grep -F "expected entry"
```

### Daemon/Service Test Template

```yaml
test:
  pipeline:
    - uses: test/tw/ldd-check
    - uses: test/daemon-check-output
      with:
        setup: |
          tee /tmp/config.yaml <<'EOF'
          server:
            port: 8080
          EOF
        start: service-name --config /tmp/config.yaml
        timeout: 60
        expected_output: "listening on"
        post: |
          set -euo pipefail
          curl -sf http://localhost:8080/health | grep -F "ok"
```

### Python Module Test Template

```yaml
test:
  pipeline:
    - uses: python/import
      with:
        python: python3
        imports: |
          import module_name
          from module_name import SomeClass
```

### How to Identify What to Test

1. **Read the project README or `--help` output** — what does the tool claim to do?
2. **Look at subcommands** — `tool help` shows available operations
3. **Check the project type** — daemon, CLI, library, plugin?
4. **Think about real use cases** — what would a user actually do?
5. **Ask: "If this test passes but the binary is broken, what did I miss?"**

## 4. Investigate Upstream (When Needed)

For functional tests you need to understand what a binary does:

```bash
# Check what binaries are in the package
ls packages/x86_64/PACKAGE-*.apk  # after local build
# Or look at the build pipeline's install targets in the YAML

# Check upstream project
gh repo view ORG/REPO
```

For finding the right `--help` or version flag:
```bash
# Common patterns to try
BINARY --help
BINARY -h
BINARY help
BINARY --version
BINARY version
BINARY -v
```

## 5. Run Tests Locally — Required Before Opening a PR

**You must run tests locally and confirm they pass before pushing or opening a PR.** Do not open a PR with untested changes. CI uses real QEMU VMs and failures there are slow and expensive to debug.

```bash
# In /home/kirkland/src/wolfi-os (the local test repo)
MELANGE_RUNNER=docker make docker-test/PACKAGE-NAME
```

If the test pipeline references `test/tw/*` pipelines not in wolfi-os, copy them:
```bash
cp /tmp/stereo/pipelines/test/tw/PIPELINE.yaml \
   /home/kirkland/src/wolfi-os/pipelines/test/tw/
```

**Do not open a PR until the local test passes.** Fix any failures first. Common issues:
- **Flaky tests**: Skip known-flaky upstream tests with `sed -i '...' + @pytest.mark.skip`
- **Container-incompatible tests**: Tests that require a real kernel (microvm, eBPF, etc.) cannot run in Docker — do not add them
- **Network-dependent tests**: Tests that call external services may be unreliable in CI

## 6. Bump the Epoch

After writing a passing test, bump the epoch by exactly **1**:

```yaml
package:
  epoch: 1  # was 0 — bump by 1 only, never more
```

**Never double-bump.** If the epoch was already bumped for another reason in this PR, do not bump again.

## 7. Checklist Before Submitting

- [ ] Read the package YAML and understood what it installs
- [ ] Chose the correct test type for the package (and each subpackage that needed it)
- [ ] Functional tests validate real behavior (not just `--help`/`--version`)
- [ ] Used `set -euo pipefail` in `runs:` blocks that contain unix pipes (omit when no pipes)
- [ ] Used `grep -F` for literal string matching, never `grep -q`
- [ ] Used `jq -e` for JSON validation
- [ ] **Tested locally with `MELANGE_RUNNER=docker make docker-test/PACKAGE` and confirmed it passes — do not open a PR until this is done**
- [ ] Epoch bumped by exactly 1
- [ ] No `|| true`, no `2>/dev/null` masking failures

## 8. Examples From This Repo

Good examples of functional tests already in the repo:

- `postgresql-15` — database creation, read/write, service startup
- `valkey-8.0` — key-value operations, Redis compatibility
- `envoy-1.33` — admin endpoints, hot restart, proxy functionality
- `prometheus-3.4` — metrics collection, rule validation
- `ssh-import-id` — help, `gh:user`, `lp:user`, `--remove` operations
