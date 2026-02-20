---
name: add-package-tests
description: Add or enhance test stanzas in melange package YAML files. Three modes: (1) Report — generate a coverage report showing untested packages, difficulty estimates, and quality gaps. (2) Coverage — get at least one test on every untested package and subpackage using simple existing pipelines. (3) Enhancement — deeply investigate a package by running it in a container, inspecting installed files, parsing help/manpage output, and discovering functional tests that actually exercise the software.
---

# Add Package Tests Skill

Three modes depending on the goal:

- **Report mode**: Generate a structured coverage report — untested packages, untested subpackages, difficulty estimates, and quality gaps in existing tests. Start here to understand scope before doing any work.
- **Coverage mode**: Find every package/subpackage missing a `test:` stanza and add the simplest correct test for each. Goal: zero untested packages.
- **Enhancement mode**: Take a package (or a class of packages) that already has minimal tests and write real functional tests — by actually running the package in a container, inspecting what it installed, and learning how to test it from its help output and man pages.

Follow CLAUDE.md "Package Test Best Practices" section throughout.

---

## Mode 0: Report — Assess Coverage and Quality

**Run all four sections immediately and in sequence without pausing for confirmation.** Produce the complete report in one pass, then present it to the user.

### Section 1: Top-Level Packages With No Tests

```bash
echo "=== TOP-LEVEL PACKAGES WITH NO TESTS ==="
grep -rL "^test:" os/*.yaml | xargs -I{} basename {} .yaml | sort
echo ""
echo "Total missing: $(grep -rL "^test:" os/*.yaml | wc -l) of $(ls os/*.yaml | wc -l)"
```

### Section 2: Subpackages With No Tests

Use PyYAML to correctly parse subpackage blocks (not pipeline step names):

```bash
echo "=== SUBPACKAGES WITH NO TESTS ==="
python3 - <<'EOF'
import glob, os, yaml

results = []
for path in sorted(glob.glob("os/*.yaml")):
    pkg = os.path.basename(path)
    try:
        data = yaml.safe_load(open(path).read())
    except Exception:
        continue
    if not data or 'subpackages' not in data:
        continue
    for sub in data.get('subpackages', []):
        if not isinstance(sub, dict):
            continue
        name = sub.get('name', '?')
        if 'test' not in sub:
            results.append(f"  {pkg}: {name}")

for r in results:
    print(r)
print(f"\nTotal subpackages missing tests: {len(results)}")
EOF
```

### Section 3: Difficulty Estimate for Each Gap

For each gap found in Sections 1 and 2, classify it using these signals from the YAML:

| Signal in YAML | Difficulty | Rationale |
|---|---|---|
| No `pipeline:` (meta, byproduct, compat) | **Trivial** | Single `uses:` line — `metapackage`, `byproductpackage`, `symlink-check` |
| `split/lib`, `split/dev`, `split/static` | **Easy** | Structural tests only — `ldd-check`, `devpackage`, `staticpackage` |
| `go/build`, `cmake/install`, `autoconf/make-install` | **Easy–Medium** | Binary with `--help`/`--version`; `ldd-check` + `help-check` |
| Config files in `/etc/`, multiple subcommands | **Medium** | Need to understand config format and invocation |
| Starts a daemon, listens on a port | **Medium–Hard** | Requires `daemon-check-output` with setup/teardown |
| `microvm`, `eBPF`, kernel modules, `/dev/` access | **Hard/Risky** | Requires real kernel — cannot run in Docker; skip for now |
| External network calls required to function | **Hard/Risky** | Unreliable in CI; mock or skip |

For each gap from Sections 1 and 2, print a one-line assessment:
```
PACKAGE/SUBPACKAGE  [trivial|easy|medium|hard|skip]  REASON
```

Example output:
```
glew                   easy      library-only, ldd-check sufficient
microvm-init           skip      requires real kernel
prometheus-cpp         easy      library, no binaries
ssh-import-id-compat   trivial   symlink package, symlink-check
```

### Section 4: Quality of Existing Tests

Scan packages that _do_ have tests and flag ones where coverage is weak:

```bash
echo "=== EXISTING TESTS — QUALITY FLAGS ==="

# Packages whose only test is byproductpackage or emptypackage (structural, no function tested)
echo "-- Structural-only (byproductpackage / emptypackage / ldd-check alone):"
grep -rl "^test:" os/*.yaml | while read f; do
    pkg=$(basename $f .yaml)
    tests=$(awk '/^test:/,/^[a-z]/' "$f" | grep -oE "uses: [a-z/A-Z_-]+" | awk '{print $2}')
    if echo "$tests" | grep -qE "^(test/tw/byproductpackage|test/tw/emptypackage|test/tw/ldd-check)$" && \
       ! echo "$tests" | grep -qvE "^(test/tw/byproductpackage|test/tw/emptypackage|test/tw/ldd-check)$"; then
        echo "  $pkg: $tests"
    fi
done

# Tests using || true or 2>/dev/null (masking failures)
echo "-- Tests masking failures (|| true or 2>/dev/null):"
grep -rl "^test:" os/*.yaml | xargs grep -l "|| true\|2>/dev/null" | xargs -I{} basename {} .yaml

# Tests with no runs: blocks at all (only uses: pipelines, possibly worth enhancing)
echo "-- Has only uses: pipelines, no custom runs: blocks:"
grep -rl "^test:" os/*.yaml | while read f; do
    pkg=$(basename $f .yaml)
    has_runs=$(awk '/^test:/,0' "$f" | grep -c "    - runs:")
    if [ "$has_runs" -eq 0 ]; then echo "  $pkg"; fi
done
```

### Summarise

After running all four sections, produce a short summary:

```
COVERAGE SUMMARY
================
Top-level packages:      NNN total,  NN missing tests  (NN%)
Subpackages:             NNN total,  NN missing tests  (NN%)

GAPS BY DIFFICULTY
==================
Trivial (single uses:):  N packages
Easy:                    N packages
Medium:                  N packages
Hard/Skip:               N packages

QUALITY FLAGS
=============
Structural-only tests:   N packages  (good candidates for enhancement)
Masking failures:        N packages  (must fix)
No functional runs::     N packages  (good candidates for enhancement)
```

---

## Mode 1: Coverage — Fill the Gaps

### Find Missing Tests

```bash
# Packages with no top-level test: section
grep -rL "^test:" os/*.yaml | sort

# Check a specific file for untested subpackages
grep -n "^  - name:\|    test:" os/PACKAGE.yaml
```

Work through untested packages systematically. For each one:
1. Read the YAML to understand what the package and each subpackage installs
2. Add a `test:` stanza using the decision tables below
3. Run locally and confirm it passes
4. Bump the epoch and commit

### Choose the Simplest Appropriate Test

**Prefer existing test pipelines over custom `runs:` blocks.** A single `uses:` line is better than a shell script to maintain.

#### Top-Level Package

| Package characteristics | Start with |
|---|---|
| Installs ELF binaries or `.so` libraries | `test/tw/ldd-check` |
| CLI tool | `test/tw/ldd-check` + `test/tw/help-check` + `test/tw/ver-check` |
| Daemon / service | `test/tw/ldd-check` + `test/daemon-check-output` |
| Meta package (no files, only runtime deps) | `test/metapackage` |
| Byproduct (lower `provider-priority`, superseded by higher-priority provider) | `test/tw/byproductpackage` |
| Intentionally empty | `test/tw/emptypackage` |
| Compat / symlink-only | `test/tw/symlink-check` |
| Python module | `python/import` |
| Ruby gem | `test/tw/gem-check` |
| Systemd service files | `test/tw/verify-service` |

#### Subpackages

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

**Byproduct / meta / empty / symlink:**
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

---

## Mode 2: Enhancement — Discover Real Functional Tests

Use this mode when a package already has minimal tests (or you've just added them) and you want to write tests that actually exercise the software's functionality. The workflow is: run the package in a container → inspect what's installed → learn from help and man pages → run candidate tests → propose what works.

### Step 1: Launch a Container With the Package Installed

```bash
# Spin up a minimal wolfi container and install the package interactively
docker run --rm -it cgr.dev/chainguard/wolfi-base sh

# Inside the container:
apk add PACKAGE-NAME
```

Or as a one-liner for scripted exploration:
```bash
docker run --rm cgr.dev/chainguard/wolfi-base sh -c "
  apk add PACKAGE-NAME 2>/dev/null
  COMMAND_HERE
"
```

### Step 2: Inspect What the Package Installed

```bash
# List all files installed by the package
apk info -L PACKAGE-NAME

# Find binaries
apk info -L PACKAGE-NAME | grep '^usr/bin/'

# Find libraries
apk info -L PACKAGE-NAME | grep '\.so'

# Find config files
apk info -L PACKAGE-NAME | grep '^etc/'

# Find man pages
apk info -L PACKAGE-NAME | grep '/man/'

# Find all subpackages that got pulled in
apk info -L PACKAGE-NAME   # shows the package itself
apk list --installed        # shows everything installed
```

This tells you exactly what to test. Make note of:
- Every binary → candidate for help, version, and functional tests
- Every `.so` → candidate for `ldd-check`
- Every man page → source of usage examples (see Step 4)
- Config files → may reveal what modes/features to test

### Step 3: Try Help and Version for Every Binary

For each binary the package installs, try common help/version flags and capture the output:

```bash
# Try all common patterns — see what the binary accepts
BINARY --help 2>&1
BINARY -h 2>&1
BINARY help 2>&1
BINARY --version 2>&1
BINARY version 2>&1
BINARY -v 2>&1
```

Read the help output carefully. Look for:
- **Subcommands** — each is a candidate for a test
- **`EXAMPLES` or `USAGE` sections** — often the richest source; try running them verbatim
- **Required arguments or config** — understand what the binary needs to run
- **Flags that reveal features** — e.g., `--format json` → test JSON output with `jq -e`

### Step 4: Mine Man Pages for Usage Examples

Man pages frequently contain `EXAMPLES` sections with real invocations. Extract them:

```bash
# Inside a container with man installed
apk add man-db PACKAGE-NAME
man BINARY 2>&1

# Or read the raw man file directly (no man command needed)
MANFILE=$(apk info -L PACKAGE-NAME | grep '/man/man')
zcat /$MANFILE   # man pages are often gzip-compressed
```

Look for the `EXAMPLES` or `SYNOPSIS` section. Each example in a man page is a candidate test case — it represents the intended usage, and if it doesn't work, the package is broken.

### Step 5: Experiment With Test Pipelines in the Container

Try running candidate test pipelines directly inside the container before writing YAML. This lets you iterate quickly without rebuilding the package.

```bash
docker run --rm cgr.dev/chainguard/wolfi-base sh -c "
  apk add PACKAGE-NAME 2>/dev/null

  # Try ldd on each binary
  for bin in /usr/bin/BINARY; do
    echo '=== ldd' \$bin '==='
    ldd \$bin
  done

  # Run a candidate functional test
  BINARY subcommand --option input-file
  echo 'exit:' \$?
"
```

If a command produces correct output and exits 0, it's a good test. If it exits non-zero or crashes, investigate why before proposing it.

### Step 6: Write and Validate the Test Stanza

Once you know which commands work and what output to expect, write the `test:` stanza:

```yaml
test:
  pipeline:
    - uses: test/tw/ldd-check
    - runs: |
        # Version check
        BINARY --version
    - runs: |
        set -euo pipefail
        # Functional test derived from --help examples or man page
        echo "sample input" | BINARY process --format json | jq -e '.result == "ok"'
        BINARY validate --file /etc/BINARY/config.yaml
        BINARY list | grep -F "expected entry"
```

**Shell script rules for `runs:` blocks:**
- Only use `set -euo pipefail` when the block contains unix pipes
- Use `grep -F` for literal string matching, never `grep -q`
- Use `jq -e` for JSON validation, not `grep`/`head`/`tail`
- Never use `|| true` or redirect errors to `/dev/null`

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
          curl -sf http://localhost:8080/metrics | jq -e '.status == "healthy"'
```

### Step 7: Run Tests Locally — Required Before Opening a PR

**You must run tests locally and confirm they pass before pushing or opening a PR.** CI uses real QEMU VMs; failures there are slow and expensive to debug.

```bash
# In /home/kirkland/src/wolfi-os (the local test repo)
MELANGE_RUNNER=docker make docker-test/PACKAGE-NAME
```

If the test pipeline references `test/tw/*` pipelines not in wolfi-os, copy them first:
```bash
cp /tmp/stereo/pipelines/test/tw/PIPELINE.yaml \
   /home/kirkland/src/wolfi-os/pipelines/test/tw/
```

**Do not open a PR until the local test passes.** Fix any failures first:
- **Flaky tests**: Skip known-flaky upstream tests with `sed -i '...' + @pytest.mark.skip`
- **Container-incompatible tests**: Tests requiring a real kernel (microvm, eBPF, etc.) cannot run in Docker — do not add them
- **Network-dependent tests**: Tests calling external services may be unreliable in CI

---

## Bump the Epoch

After writing a passing test, bump the epoch by exactly **1**:

```yaml
package:
  epoch: 1  # was 0 — bump by exactly 1, never more
```

**Never double-bump.** If the epoch was already bumped in this PR, do not bump again.

---

## Checklist Before Submitting

**Coverage:**
- [ ] Every top-level package in scope has a `test:` stanza
- [ ] Every subpackage in scope has a `test:` stanza
- [ ] Used the simplest existing test pipeline appropriate for each package type

**Enhancement:**
- [ ] Inspected installed files with `apk info -L` to know what the package actually contains
- [ ] Tried `--help`, `-h`, `--version` for every installed binary
- [ ] Read man pages and/or `EXAMPLES` sections for usage patterns
- [ ] Candidate tests were run inside a container and confirmed working before writing YAML
- [ ] Tests exercise real functionality, not just `--help`/`--version` alone

**All tests:**
- [ ] Custom `runs:` blocks use `set -euo pipefail` only when pipes are present
- [ ] `grep -F` used for literal string matching, never `grep -q`
- [ ] `jq -e` used for JSON validation
- [ ] **Tested locally with `MELANGE_RUNNER=docker make docker-test/PACKAGE` and confirmed passing — do not open a PR until this is done**
- [ ] Epoch bumped by exactly 1
- [ ] No `|| true`, no `2>/dev/null` masking failures

---

## Examples From This Repo

- `ssh-import-id` — help, `gh:user`, `lp:user`, `--remove` operations
- `postgresql-15` — database creation, read/write, service startup
- `valkey-8.0` — key-value operations, Redis compatibility
- `envoy-1.33` — admin endpoints, hot restart, proxy functionality
- `prometheus-3.4` — metrics collection, rule validation
