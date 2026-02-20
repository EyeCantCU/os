---
name: add-package-tests
description: Add top-level test stanzas to melange package YAML files that are missing them. The primary goal is complete test coverage — every package and every subpackage must have at least one test. Start with simple existing test pipelines (ldd-check, help-check, ver-check, etc.) before writing custom functional tests.
---

# Add Package Tests Skill

The primary goal is **complete test coverage**: every package and every subpackage in the stereo repo must have at least one `test:` stanza. Start by finding untested packages, then apply the simplest appropriate test. Only write custom `runs:` blocks when no existing pipeline fits.

Follow CLAUDE.md "Package Test Best Practices" section throughout.

## 1. Find Packages and Subpackages Missing Tests

```bash
# Find package YAMLs with no top-level test: section at all
grep -rL "^test:" os/*.yaml | sort

# For a specific file, check whether each subpackage has a test: block
grep -n "^  - name:\|test:" os/PACKAGE.yaml
```

Work through untested packages systematically. For each one:
1. Read the YAML to understand what the package and each subpackage installs
2. Add a `test:` stanza to the package and to any subpackage that lacks one
3. Run the test locally and confirm it passes
4. Bump the epoch and commit

## 2. Understand the Package

Read the YAML and answer these questions for the top-level package **and each subpackage**:

- Does it install **ELF binaries** or **`.so` libraries**? → `ldd-check`
- Is it a **CLI tool** with `--help` / `--version`? → `help-check`, `ver-check`
- Is it a **daemon or service**? → `daemon-check-output`
- Is it a **meta package** (only `dependencies.runtime`, no installed files)? → `metapackage`
- Is it a **byproduct** (lower `provider-priority`, superseded by a higher-priority provider)? → `byproductpackage`
- Is it **intentionally empty** (only `spdx.json`)? → `emptypackage`
- Is it a **compat / symlink-only** package? → `symlink-check`
- Does it install a **Python module**? → `python/import`
- Is it a **Ruby gem**? → `gem-check`
- Does it install **systemd service files**? → `verify-service`

## 3. Choose the Simplest Appropriate Test

**Prefer existing test pipelines over custom `runs:` blocks.** A single `uses:` line is better than a shell script you have to maintain.

### Top-Level Package Decision Table

| Package characteristics | Start with |
|---|---|
| Installs ELF binaries or `.so` libraries | `test/tw/ldd-check` |
| CLI tool | `test/tw/ldd-check` + `test/tw/help-check` + `test/tw/ver-check` |
| Daemon / service | `test/tw/ldd-check` + `test/daemon-check-output` |
| Meta package (no files, only runtime deps) | `test/metapackage` |
| Byproduct (lower provider-priority) | `test/tw/byproductpackage` |
| Intentionally empty | `test/tw/emptypackage` |
| Compat / symlink-only | `test/tw/symlink-check` |
| Python module | `python/import` |
| Ruby gem | `test/tw/gem-check` |
| Systemd service files | `test/tw/verify-service` |

### Subpackage Decision Table

| Subpackage name/type | Start with |
|---|---|
| `-dev` with `.pc` files | `test/pkgconf` + `test/tw/ldd-check` |
| `-dev` C/C++ headers | `test/tw/header-check` + `test/tw/devpackage` |
| `-libs` | `test/tw/ldd-check` |
| `-static` | `test/tw/staticpackage` |
| `-doc` | `test/tw/docs` |
| `-compat` | `test/tw/symlink-check` |
| `-debug` | `test/tw/debugpackage` |

### Minimal Test Examples

**Binary package:**
```yaml
test:
  pipeline:
    - uses: test/tw/ldd-check
```

**CLI tool:**
```yaml
test:
  pipeline:
    - uses: test/tw/ldd-check
    - uses: test/tw/help-check
      with:
        bins: tool-name
    - uses: test/tw/ver-check
      with:
        bins: tool-name
        version: ${{package.version}}
```

**Byproduct / meta / empty / symlink packages:**
```yaml
test:
  pipeline:
    - uses: test/tw/byproductpackage   # or metapackage / emptypackage / symlink-check
```

**Python module:**
```yaml
test:
  pipeline:
    - uses: python/import
      with:
        python: python3
        imports: |
          import module_name
```

## 4. Add Functional Tests (When a Simple Pipeline Isn't Enough)

Once a package has at least one test, consider whether it exercises real behavior. `ldd-check` alone for a CLI tool is a weak test — add a `runs:` block that actually invokes the binary.

**Shell script rules for `runs:` blocks:**

```bash
# Only use set -euo pipefail when the block contains unix pipes
set -euo pipefail
tool-name list | grep -F "expected entry"

# Omit it entirely when there are no pipes
tool-name --version
tool-name validate --file /etc/tool/config.yaml

# Use grep -F for literal strings, never grep -q
some_command | grep -F "expected string"

# Use jq -e for JSON — not grep/head/tail
curl -sf http://localhost:8080/api | jq -e '.status == "ok"'
```

**CLI tool with functional test:**
```yaml
test:
  pipeline:
    - uses: test/tw/ldd-check
    - uses: test/tw/ver-check
      with:
        bins: tool-name
        version: ${{package.version}}
    - runs: |
        tool-name --help
        tool-name validate --file /etc/tool/config.yaml
    - runs: |
        set -euo pipefail
        tool-name list | grep -F "expected entry"
```

**Daemon/service:**
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

## 5. Run Tests Locally — Required Before Opening a PR

**You must run tests locally and confirm they pass before pushing or opening a PR.** Do not open a PR with untested changes. CI uses real QEMU VMs and failures there are slow and expensive to debug.

```bash
# In /home/kirkland/src/wolfi-os (the local test repo)
MELANGE_RUNNER=docker make docker-test/PACKAGE-NAME
```

If the test pipeline references `test/tw/*` pipelines not in wolfi-os, copy them first:
```bash
cp /tmp/stereo/pipelines/test/tw/PIPELINE.yaml \
   /home/kirkland/src/wolfi-os/pipelines/test/tw/
```

**Do not open a PR until the local test passes.** Fix any failures first. Common issues:
- **Flaky tests**: Skip known-flaky upstream tests with `sed -i '...' + @pytest.mark.skip`
- **Container-incompatible tests**: Tests requiring a real kernel (microvm, eBPF, etc.) cannot run in Docker — do not add them
- **Network-dependent tests**: Tests that call external services may be unreliable in CI

## 6. Bump the Epoch

After writing a passing test, bump the epoch by exactly **1**:

```yaml
package:
  epoch: 1  # was 0 — bump by exactly 1, never more
```

**Never double-bump.** If the epoch was already bumped for another reason in this PR, do not bump again.

## 7. Checklist Before Submitting

- [ ] Every top-level package in scope has a `test:` stanza
- [ ] Every subpackage in scope has a `test:` stanza
- [ ] Used the simplest existing test pipeline that makes sense for each package type
- [ ] Custom `runs:` blocks use `set -euo pipefail` only when pipes are present
- [ ] Used `grep -F` for literal string matching, never `grep -q`
- [ ] Used `jq -e` for JSON validation
- [ ] **Tested locally with `MELANGE_RUNNER=docker make docker-test/PACKAGE` and confirmed it passes — do not open a PR until this is done**
- [ ] Epoch bumped by exactly 1
- [ ] No `|| true`, no `2>/dev/null` masking failures

## 8. Examples From This Repo

Good examples of tests already in the repo:

- `ssh-import-id` — help, `gh:user`, `lp:user`, `--remove` operations
- `postgresql-15` — database creation, read/write, service startup
- `valkey-8.0` — key-value operations, Redis compatibility
- `envoy-1.33` — admin endpoints, hot restart, proxy functionality
- `prometheus-3.4` — metrics collection, rule validation
