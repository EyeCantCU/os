---
name: package-tests
description: Manage test stanzas in melange package YAML files. Four modes: (1) Report — generate a coverage report showing untested packages, difficulty estimates, and quality gaps. (2) Coverage — get at least one test on every untested package and subpackage using simple existing pipelines. (3) Enhancement — deeply investigate a package by running it in a container, inspecting installed files, parsing help/manpage output, and discovering functional tests that actually exercise the software. (4) New Pipelines — propose and create reusable test pipelines for patterns that appear across many packages.
allowed-tools:
  - Bash
  - Read
  - Edit
  - Write
  - Glob
  - Grep
---

# Package Tests Skill

Four modes depending on the goal:

- **Report mode**: Generate a structured coverage report — untested packages, untested subpackages, difficulty estimates, and quality gaps in existing tests. Start here to understand scope before doing any work.
- **Coverage mode**: Find every package/subpackage missing a `test:` stanza and add the simplest correct test for each. Goal: zero untested packages.
- **Enhancement mode**: Take a package (or a class of packages) that already has minimal tests and write real functional tests — by actually running the package in a container, inspecting what it installed, and learning how to test it from its help output and man pages.
- **New Pipelines mode**: Propose and create reusable test pipelines for patterns that appear across many packages.

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

### Section 5: Pattern Analysis — Candidates for New Pipelines

Look for repeated test patterns across untested subpackages that don't yet have a dedicated pipeline. The goal: if 10+ packages need the same test logic, that logic belongs in a reusable pipeline under `os/pipelines/test/tw/`, not copy-pasted into each YAML.

```bash
python3 - <<'EOF'
import glob, os, yaml, re
from collections import defaultdict

# Known existing pipelines for each suffix
existing = {
    '-static':          'test/tw/staticpackage',
    '-doc':             'test/tw/docs',
    '-dbg':             'test/tw/debugpackage',
    '-debug':           'test/tw/debugpackage',
    '-dev':             'test/pkgconf / test/tw/devpackage',
    '-libs':            'test/tw/ldd-check',
    '-compat':          'test/tw/symlink-check',
    '-openrc':          'test/tw/verify-service',
}

suffix_patterns = [
    '-bash-completion', '-zsh-completion', '-fish-completion',
    '-openrc', '-lang', '-config', '-compat', '-static',
    '-doc', '-dbg', '-debug', '-dev', '-libs', '-src',
]

counts = defaultdict(list)
for path in sorted(glob.glob("os/*.yaml")):
    pkg = os.path.basename(path)
    try:
        data = yaml.safe_load(open(path).read())
    except:
        continue
    if not data or 'subpackages' not in data:
        continue
    for sub in data.get('subpackages', []):
        if not isinstance(sub, dict) or 'test' in sub:
            continue
        name = str(sub.get('name', ''))
        matched = False
        for suffix in suffix_patterns:
            if suffix in name:
                counts[suffix].append((pkg, name))
                matched = True
                break
        if not matched:
            counts['_other'].append((pkg, name))

print(f"{'Pattern':<25} {'Untested':>8}  {'Existing pipeline or recommendation'}")
print("-" * 80)
for suffix in suffix_patterns + ['_other']:
    items = counts.get(suffix, [])
    if not items:
        continue
    if suffix in existing:
        note = f"{existing[suffix]} ✓  (just needs applying)"
    elif suffix in ('-bash-completion', '-zsh-completion', '-fish-completion'):
        note = "— RECOMMEND new: test/tw/shell-completion-check"
    elif suffix == '-lang':
        note = "— RECOMMEND new: test/tw/langpackage (check .mo/.po files exist)"
    elif suffix == '-config':
        note = "— RECOMMEND new: test/tw/configpackage (check config files exist, are valid)"
    elif suffix == '-src':
        note = "— RECOMMEND new: test/tw/srcpackage or test/tw/contains-files"
    else:
        note = "— inspect individually"
    print(f"  {suffix:<23} {len(items):>8}  {note}")
EOF
```

For each "RECOMMEND new" entry, the pattern suggests creating a new pipeline at `os/pipelines/test/tw/PIPELINE-NAME.yaml`. See **Mode 3: Propose New Pipelines** for how to do this.

Also mine **existing custom `runs:` blocks** for repeated patterns — if many packages share the same shell logic in their test stanzas, that logic is a pipeline waiting to be extracted:

```bash
# Find the most common multi-line runs: patterns in test blocks
python3 - <<'EOF'
import glob, yaml, re
from collections import Counter

snippets = Counter()
for path in glob.glob("os/*.yaml"):
    try:
        content = open(path).read()
    except:
        continue
    # Extract runs: blocks inside test: sections
    in_test = False
    for line in content.split('\n'):
        if line.startswith('test:'):
            in_test = True
        elif line and not line[0].isspace():
            in_test = False
        if in_test and '- runs: |' in line:
            # Grab the next few lines as a fingerprint
            pass  # extend if needed

# Simpler: look for common command patterns in test blocks
patterns = Counter()
for path in glob.glob("os/*.yaml"):
    content = open(path).read()
    # Find test block
    m = re.search(r'^test:.*?(?=^\w|\Z)', content, re.MULTILINE | re.DOTALL)
    if not m:
        continue
    block = m.group(0)
    for cmd in re.findall(r'(?:ldd|--help|-h|--version|-v|apk info|stat /)', block):
        patterns[cmd] += 1

print("Common commands in existing test blocks:")
for cmd, n in patterns.most_common(15):
    print(f"  {n:>5}x  {cmd}")
EOF
```

### Summarise

After running all five sections, produce a short summary:

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
Masking failures:        N packages  (must fix — remove || true / 2>/dev/null)

PIPELINE OPPORTUNITIES
======================
Patterns with existing pipeline not yet applied:  N subpackages
Patterns that warrant a NEW pipeline:             N subpackages
  (list each recommended new pipeline with count)
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

### Step 5: Check README Files in the Source Repository

After installing the package, check the upstream source repository's README for usage examples. The melange YAML points to the source via `git-checkout` or `fetch` — use that URL to find the repo, then look for README files:

```bash
# Clone or browse the upstream repo (URL from the yaml's git-checkout/fetch step)
REPO_URL=$(grep -A2 'git-checkout\|fetch' os/PACKAGE.yaml | grep 'repository:\|uri:' | head -1 | awk '{print $2}')

# Clone and look for README files
git clone --depth=1 "$REPO_URL" /tmp/pkg-src
ls /tmp/pkg-src/README* /tmp/pkg-src/readme* 2>/dev/null

# Read them and extract usage examples
cat /tmp/pkg-src/README.md 2>/dev/null | head -200
```

README files often contain:
- **Quick-start examples** — the simplest invocations, ideal as smoke tests
- **Feature demonstrations** — show the core value of the tool
- **Configuration snippets** — reveal what config options to exercise
- **CLI examples** — often more accessible than man pages

Look specifically for code blocks (triple-backtick fences) with shell commands. Each runnable example is a candidate test case. Prefer examples that don't require external services or network access.

### Step 6: Experiment With Test Pipelines in the Container

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

### Step 7: Write and Validate the Test Stanza

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

**Before writing a `runs:` block, ask: does this logic belong in a pipeline?**

If you find yourself writing the same test logic for more than one package — or if the logic is purely structural (check files exist, check format, check syntax) and other packages of the same type would clearly benefit — **stop and create a reusable pipeline** at `os/pipelines/test/tw/` first, then use it with a single `uses:` line. See **Mode 3: Propose and Create New Test Pipelines**.

Examples of logic that became (or should become) pipelines:
- Validating all `.xml` files with `xmlwf` → `test/tw/xml-syntax-check`
- Checking shell completion files exist → `test/tw/shell-completion-check` (proposed)
- Checking config files are present → `test/tw/configpackage` (proposed)

**Shell script rules for `runs:` blocks (when a pipeline genuinely doesn't fit):**
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

### Step 8: Run Tests Locally — Required Before Opening a PR

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

## PR Workflow — Always Use the Fork

Branches live on the fork (your personal GitHub fork of `chainguard-dev/stereo`). PRs always target `chainguard-dev/stereo` (upstream). **Never open a PR directly against the fork.**

```bash
# Detect your GitHub username from the gh CLI
GITHUB_ID=$(gh api user --jq .login)

# Push branch to fork
git push -u origin BRANCH-NAME

# Open PR against upstream from fork branch
gh pr create --repo chainguard-dev/stereo --base main \
  --head "${GITHUB_ID}:BRANCH-NAME" --title "..." --body "..."
```

Do **not** use `gh pr create` without `--repo chainguard-dev/stereo` — that will open the PR against the fork instead of upstream.

---

## Bump the Epoch

After writing a passing test, bump the epoch by exactly **1**:

```yaml
package:
  epoch: 1  # was 0 — bump by exactly 1, never more
```

**Never double-bump.** If the epoch was already bumped in this PR, do not bump again.

> **Note:** Strictly speaking, bumping the epoch is not required when the only change is adding a `test:` stanza — tests don't affect the built package artifacts. However, it is good hygiene to bump anyway: it forces CI to rebuild the package from source and run the full build-and-test pipeline end-to-end, which pressure-tests the entire chain and surfaces hard-to-debug regressions that might otherwise lay dormant until a future unrelated change triggers a rebuild. If the driving user prefers to skip the epoch bump to avoid a rebuild, that is a valid choice — just be aware of the trade-off.

---

## Checklist Before Submitting

**Coverage:**
- [ ] Every top-level package in scope has a `test:` stanza
- [ ] Every subpackage in scope has a `test:` stanza
- [ ] Used the simplest existing test pipeline appropriate for each package type
- [ ] If the same `runs:` logic appears (or would appear) in more than one package, extracted it into a reusable pipeline under `os/pipelines/test/tw/` instead

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

## Mode 3: Propose and Create New Test Pipelines

Use this mode when the Report (Section 5) identifies a pattern where 10+ packages share the same untested structure and no existing pipeline covers it. Rather than copy-pasting the same `runs:` block into dozens of YAMLs, create a reusable pipeline once.

### When to Create a New Pipeline

A new pipeline is warranted when:
- **10+ subpackages** share the same name suffix and the same required test logic
- The test logic is **purely structural** (check files exist, check format, check no broken symlinks) — not package-specific
- No existing pipeline already covers the pattern

Current top candidates based on the report (check Section 5 output for current counts):

| Pattern | Suggested pipeline | What it should check |
|---|---|---|
| `-bash-completion`, `-zsh-completion`, `-fish-completion` | `test/tw/shell-completion-check` | Completion files exist under `/usr/share/bash-completion/`, `/usr/share/zsh/`, or `/usr/share/fish/` |
| `-lang` | `test/tw/langpackage` | `.mo` or locale files exist under `/usr/share/locale/` |
| `-config` | `test/tw/configpackage` | Config files exist under `/etc/`; are non-empty |

### Pipeline File Format

Pipelines live at `os/pipelines/test/tw/PIPELINE-NAME.yaml`. Use an existing one as a reference:

```yaml
name: Shell Completion Check

description: |
  Validates that a package installs shell completion files in the
  correct locations for bash, zsh, or fish.

needs:
  packages:
    - busybox  # or a dedicated checker tool if needed

inputs:
  shell:
    description: "Shell to check completions for: bash, zsh, or fish"
    default: ""

pipeline:
  - name: Check shell completion files are installed
    runs: |
      # Detect which shell(s) this package provides completions for
      pkg="${{context.name}}"
      found=0
      for dir in \
          usr/share/bash-completion/completions \
          usr/share/zsh/site-functions \
          usr/share/fish/vendor_completions.d; do
        if apk info -L "$pkg" | grep -qF "$dir/"; then
          echo "Found completions in $dir"
          found=1
        fi
      done
      if [ "$found" -eq 0 ]; then
        echo "ERROR: no completion files found for $pkg"
        exit 1
      fi
```

### Workflow

1. **Verify the pattern is real** — run the Section 5 script, confirm 10+ instances
2. **Pick one example package** — read its YAML, install it in a container, inspect the files
3. **Draft the pipeline** — write `os/pipelines/test/tw/PIPELINE-NAME.yaml`
4. **Test it against several packages** — apply it in a few YAMLs, run `MELANGE_RUNNER=docker make docker-test/PACKAGE` for each
5. **Apply at scale** — once validated, add `uses: test/tw/PIPELINE-NAME` to all matching subpackages in a single PR
6. **Update this skill** — add the new pipeline to the decision tables in Modes 0–2

### After Creating a Pipeline

- Add it to the **Section 5 pattern table** in the Report script (`existing` dict)
- Add it to the **decision tables** in Mode 1 (Coverage) and Mode 2 (Enhancement)
- Add it to CLAUDE.md's "Use test pipelines" section so all contributors know it exists

---

## Examples From This Repo

- `ssh-import-id` — help, `gh:user`, `lp:user`, `--remove` operations
- `postgresql-15` — database creation, read/write, service startup
- `valkey-8.0` — key-value operations, Redis compatibility
- `envoy-1.33` — admin endpoints, hot restart, proxy functionality
- `prometheus-3.4` — metrics collection, rule validation
