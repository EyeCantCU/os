---
name: fetch-to-checkout
description: Convert melange packages from `fetch` (tarball download) to `git-checkout`. Use when migrating packages in os/*.yaml or enterprise-packages/*.yaml from release-tarball sourcing to direct git tag checkout. Follows the patterns established by Colin Watson (cjwatson) in ~80 conversion PRs.
allowed-tools:
  - Bash
  - Read
  - Edit
  - Write
  - Glob
  - Grep
---

# Fetch-to-Checkout Skill

Converts melange packages from `uses: fetch` (tarball downloads) to `uses: git-checkout` (direct git tag checkout). This improves reproducibility, enables git-based automation, and allows the `github:` update poller to track releases directly.

## Execution Philosophy: Announce Once, Then Act

Print a one-time plan listing every file to be edited, then execute without further interruption. Never ask "shall I continue?" mid-flow.

---

## Step 1: Identify What to Convert

The user may specify:
- A single package: `haproxy-2.8`
- A glob: `py3-a*`
- A list: `htop`, `harfbuzz`, `linux-pam`
- "all remaining fetch packages" → run the scanner

**Find all packages still using fetch:**
```bash
python3 - <<'EOF'
import glob, yaml, os
for path in sorted(glob.glob("os/*.yaml")):
    try:
        data = yaml.safe_load(open(path).read())
    except:
        continue
    if not data:
        continue
    pipeline = data.get('pipeline', []) or []
    uses_fetch = any(
        isinstance(step, dict) and step.get('uses') == 'fetch'
        for step in pipeline
    )
    if uses_fetch:
        # Extract the fetch URI for context
        for step in pipeline:
            if isinstance(step, dict) and step.get('uses') == 'fetch':
                uri = (step.get('with') or {}).get('uri', '')
                print(f"{os.path.basename(path):<50} {uri[:80]}")
EOF
```

---

## Step 2: For Each Package — Read and Analyze

Read the YAML file. Extract:
1. **`fetch.uri`** — tells you where the tarball came from → reveals the upstream repo
2. **`fetch.expected-sha256`** — not needed after conversion (replaced by `expected-commit`)
3. **`package.version`** — used to construct the tag
4. **`update:` section** — will need updating

### Detecting the Upstream Repository and Tag

The fetch URI almost always reveals the git repository and tag. Work through these patterns in order:

#### Pattern A: GitHub releases or archive download
```
https://github.com/OWNER/REPO/releases/download/TAG/name-VERSION.tar.gz
https://github.com/OWNER/REPO/archive/TAG.tar.gz
https://github.com/OWNER/REPO/archive/refs/tags/TAG.tar.gz
```
→ **repository:** `https://github.com/OWNER/REPO`
→ **tag:** extract TAG directly (may be `v${{package.version}}`, `${{package.version}}`, `OWNER-${{package.version}}`, etc.)

Examples:
- `https://github.com/htop-dev/htop/releases/download/3.4.1/htop-3.4.1.tar.xz` → repo: `https://github.com/htop-dev/htop`, tag: `${{package.version}}`
- `https://github.com/erlang/otp/releases/download/OTP-25.3.2.21/otp_src_25.3.2.21.tar.gz` → repo: `https://github.com/erlang/otp`, tag: `OTP-${{package.version}}`
- `https://github.com/linux-pam/linux-pam/releases/download/v1.7.1/Linux-PAM-1.7.1.tar.xz` → repo: `https://github.com/linux-pam/linux-pam`, tag: `v${{package.version}}`
- `https://github.com/cython/cython/archive/3.2.4.tar.gz` → repo: `https://github.com/cython/cython`, tag: `${{package.version}}`
- `https://github.com/harfbuzz/harfbuzz/releases/download/12.2.0/harfbuzz-12.2.0.tar.xz` → repo: `https://github.com/harfbuzz/harfbuzz`, tag: `${{package.version}}`

#### Pattern B: PyPI
```
https://files.pythonhosted.org/packages/source/X/PYPI-NAME/PYPI-NAME-VERSION.tar.gz
```
→ Search the package's PyPI page or GitHub to find the upstream repo.
→ Use `gh api repos/OWNER/REPO` or check the PyPI metadata.
→ Most Python packages tag with `v${{package.version}}` on GitHub.

To find the repo for a PyPI package:
```bash
# Try PyPI JSON API
curl -s "https://pypi.org/pypi/PACKAGE-NAME/json" | python3 -c "
import sys, json
d = json.load(sys.stdin)
urls = d.get('info', {})
print(urls.get('home_page', ''))
print(urls.get('project_urls', {}).get('Source', ''))
print(urls.get('project_urls', {}).get('Repository', ''))
"
```

#### Pattern C: GNU mirrors
```
https://ftpmirror.gnu.org/gnu/PACKAGE/PACKAGE-VERSION.tar.gz
https://ftp.gnu.org/gnu/PACKAGE/PACKAGE-VERSION.tar.gz
```
→ Check if the package is on Savannah git: `https://git.savannah.gnu.org/git/PACKAGE.git`
→ Tag is usually `v${{package.version}}`
→ **Important:** git.savannah.gnu.org is flaky — if the package uses gnulib as a submodule, redirect it to GitHub's mirror (see Bootstrap section below)

#### Pattern D: Savannah non-GNU / other hosting
```
https://download.savannah.nongnu.org/releases/PACKAGE/PACKAGE-VERSION.tar.gz
```
→ Check https://savannah.nongnu.org/projects/PACKAGE for the git URL
→ May be at `https://savannah.nongnu.org/git/?group=PACKAGE`

#### Pattern E: Other registries (PECL, CPAN, etc.)
- **PECL** (`https://pecl.php.net/get/PACKAGE-VERSION.tgz`): find the GitHub repo (usually listed on the PECL package page or in the package's source)
- **CPAN** (`https://cpan.metacpan.org/...`): find the GitHub/git repo from the module's metacpan page

#### Pattern F: GitLab / Codeberg / other git hosts
Some packages live on GitLab or Codeberg. cjwatson has used:
- `https://gitlab.com/NAMESPACE/PROJECT` (libpipeline, man-db)
- `https://codeberg.org/USER/REPO` (chrpath)

---

## Step 3: Resolve the expected-commit

Once you know the repository and tag, resolve the commit hash:

```bash
# For annotated tags, ^{} dereferences to the commit
git ls-remote --tags REPO_URL "refs/tags/TAG^{}" 2>/dev/null | awk '{print $1}'

# If that gives nothing, try without dereference (lightweight tag)
git ls-remote --tags REPO_URL "refs/tags/TAG" 2>/dev/null | awk '{print $1}'
```

Example:
```bash
git ls-remote --tags https://github.com/htop-dev/htop "refs/tags/3.4.1^{}" | awk '{print $1}'
# → 348c0a6bf4f33571835a0b6a1a0f5deb15132128
```

**If the tag doesn't exist yet** (version is very new), use `git ls-remote` to list all tags and identify the closest match, or clone and check:
```bash
git ls-remote --tags REPO_URL | grep "VERSION" | head -10
```

---

## Step 4: Determine Build System Changes

Building from git (not a tarball) sometimes requires extra build-time dependencies because release tarballs include pre-generated `configure` scripts.

### GNU autoconf/automake projects (Savannah, GNU mirrors)

Tarballs include pre-generated `configure`. Git checkouts do not.

**Signs you need bootstrap:** The project uses autoconf/automake (has `configure.ac`, `Makefile.am`).

**Prefer the `autoconf/configure` pipeline** (if the package isn't using it already) — it already pulls in `autoconf`, `automake`, and related deps automatically. Only add them manually to `environment.contents.packages` if you can't switch to that pipeline:
```yaml
- autoconf
- automake
```

**Add a bootstrap step after git-checkout:**

For projects with gnulib submodule (very common for GNU tools: sed, bison, cpio, findutils, etc.):
```yaml
- name: git.savannah.gnu.org is flaky
  runs: git submodule set-url gnulib https://github.com/coreutils/gnulib.git

- runs: echo ${{package.version}} >.tarball-version

- runs: ./bootstrap
```

Some need `wget` or `rsync` for gnulib translations:
```yaml
- wget # for gnulib translations   # in environment packages
```

For projects with gnulib via `GNULIB_URL` (man-db, libpipeline):
```yaml
- runs: GNULIB_URL=https://github.com/coreutils/gnulib.git ./bootstrap
```

**Additional deps often needed:**
```yaml
- flex          # if configure.ac uses AC_PROG_LEX
- gettext-dev   # if project uses gettext/translations
- gperf         # if project uses gperf
- help2man      # if project builds man pages at configure time
- libtool       # many autoconf projects
- pkgconf-dev   # for pkg-config in bootstrap
- python3       # some projects use python in build scripts
- texinfo       # for info pages
- wget          # for gnulib translations
```

### Projects with `autogen.sh`

```yaml
- runs: ./autogen.sh
```

### Projects with `./bootstrap`

```yaml
- runs: ./bootstrap
```

### Go / Meson / CMake projects

Usually no extra bootstrap needed — they build cleanly from git.

### Python projects (hatchling, setuptools, etc.)

Some Python packages on GitHub don't include `.tarball-version` or generated files. Most build fine from git. Some require `py3-supported-hatch-vcs` added to build deps for SCM-based versioning.

---

## Step 5: Update the `update:` Section

### From `release-monitor` to `github:`

**Before:**
```yaml
update:
  enabled: true
  release-monitor:
    identifier: 12345
```

**After (tag is `v${{package.version}}`, i.e., `v1.2.3`):**
```yaml
update:
  enabled: true
  github:
    identifier: OWNER/REPO
    strip-prefix: v
    tag-filter-prefix: v
```

**After (tag is bare `${{package.version}}`, i.e., `1.2.3`):**
```yaml
update:
  enabled: true
  github:
    identifier: OWNER/REPO
```
(No `strip-prefix`, no `tag-filter-prefix`)

**After (tag uses a custom prefix, e.g., `release-1.2.3`):**
```yaml
update:
  enabled: true
  github:
    identifier: OWNER/REPO
    strip-prefix: release-
    tag-filter-prefix: release-
```

**After (tag uses `OTP-` prefix like erlang):**
```yaml
update:
  enabled: true
  github:
    identifier: erlang/otp
    strip-prefix: OTP-
    tag-filter-prefix: OTP-
```

### When to keep `release-monitor:`

**CRITICAL: The wolfictl linter enforces `git-checkout-must-use-github-updates` — if a package uses `uses: git-checkout`, its `update:` section MUST use `github:` or `git:`, never `release-monitor:`. Failing to do this will cause a lint ERROR in CI.**

This means:
- If the upstream IS on GitHub → switch `release-monitor:` to `github:` (required, not optional)
- If the upstream is NOT on GitHub (savannah, gitlab, codeberg, etc.) → switch `release-monitor:` to `git:` pointing to the upstream git URL, or reconsider whether to convert the package at all

For non-GitHub hosts the `git:` update block looks like:
```yaml
update:
  enabled: true
  git:
    tag-filter-prefix: v  # if tags have a prefix
```

### When to disable updates

If the package is deprecated or the version tracking is complex, set `enabled: false` with an `exclude-reason:`. cjwatson used this for `py3-avro-python3` which has an unusual tag scheme.

### When to re-enable updates

If `enabled: false` existed with `exclude-reason: Code is not in a SCM for us to use git poller` or similar — now that we're using git, re-enable it:
```yaml
update:
  enabled: true
  github:
    identifier: OWNER/REPO
```

---

## Step 6: Make the Edit

### The Minimal Change (GitHub project, no autoconf bootstrap needed)

```yaml
# BEFORE:
pipeline:
  - uses: fetch
    with:
      expected-sha256: <hash>
      uri: https://github.com/OWNER/REPO/releases/download/v${{package.version}}/NAME-${{package.version}}.tar.gz

# AFTER:
pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/OWNER/REPO
      tag: v${{package.version}}
      expected-commit: <resolved-commit-hash>
```

Also update the `update:` section and bump the epoch.

### The Full Change (autoconf GNU project with gnulib)

```yaml
# Environment: add autoconf, automake, wget, and any other needed deps

# BEFORE:
pipeline:
  - uses: fetch
    with:
      expected-sha256: <hash>
      uri: https://ftpmirror.gnu.org/gnu/PKG/PKG-${{package.version}}.tar.gz

# AFTER:
pipeline:
  - uses: git-checkout
    with:
      repository: https://git.savannah.gnu.org/git/PKG.git
      tag: v${{package.version}}
      expected-commit: <resolved-commit-hash>

  - name: git.savannah.gnu.org is flaky
    runs: git submodule set-url gnulib https://github.com/coreutils/gnulib.git

  - runs: echo ${{package.version}} >.tarball-version

  - runs: ./bootstrap
```

---

## Step 7: Bump the Epoch

Increment `epoch` by exactly 1 **relative to `origin/main`**, not relative to what was in the file when you started.

**CRITICAL: Always check the current epoch on `origin/main` before committing**, because main advances continuously and may have bumped the same package since you branched (e.g. for a CVE fix):

```bash
git show origin/main:os/PACKAGE.yaml | grep "^  epoch:"
```

Your epoch must be strictly greater than what's on main. If main has caught up (both at the same value), increment again.

**Before pushing, verify all changed files have a strictly higher epoch than main:**

```bash
for f in $(git diff origin/main...HEAD --name-only -- 'os/*.yaml'); do
  our=$(grep "^  epoch:" $f | head -1 | awk '{print $2}')
  main=$(git show origin/main:$f 2>/dev/null | grep "^  epoch:" | head -1 | awk '{print $2}')
  if [ "${our:-0}" -le "${main:-0}" ] 2>/dev/null; then
    echo "MISSING BUMP: $f (main=$main, ours=$our)"
  else
    echo "OK: $f (main=$main -> ours=$our)"
  fi
done
```

If any show `MISSING BUMP`, increment that package's epoch further before committing.

---

## Step 8: Lint

After editing, run lint from the appropriate subdirectory:

```bash
# For os/ packages:
(cd os && ./lint.sh PACKAGE.yaml)

# For enterprise-packages/:
(cd enterprise-packages && ./lint.sh PACKAGE.yaml)
```

Fix any issues before proceeding.

---

## Step 9: Build and Test Locally

**Always build and test before committing.** Do not open a PR without a passing local build and test.

```bash
make package/PACKAGE-NAME
make test/PACKAGE-NAME
```

Both must exit 0. The QEMU shutdown message `ERRO Failed to run command "sh -c 'echo s > /proc/sysrq-trigger..."` is normal and not a failure — it is just the VM shutting down abruptly. Ignore it.

### Common Build Failures After fetch→git-checkout Conversion

#### Missing `configure` script
```
./configure: No such file or directory
```
The tarball included a pre-generated `configure`; the git checkout does not.

**Fix:** Add bootstrap steps and build deps. For autoconf projects:
```yaml
# In environment.contents.packages, add:
- autoconf
- automake

# After git-checkout in pipeline, add:
- runs: ./autogen.sh   # or ./bootstrap, depending on the project
```

For GNU projects using gnulib as a submodule:
```yaml
- name: git.savannah.gnu.org is flaky
  runs: git submodule set-url gnulib https://github.com/coreutils/gnulib.git

- runs: echo ${{package.version}} >.tarball-version

- runs: ./bootstrap
```

#### Wrong version detected
```
configure: error: cannot determine version
```
or the build embeds `0.0.0` instead of the real version.

**Fix:** Write `.tarball-version` so autoconf version detection works:
```yaml
- runs: echo ${{package.version}} >.tarball-version
```

Some projects also need `echo ${{package.version}} > .version`.

#### Missing submodule content
```
fatal: No url found for submodule path 'SUBMODULE'
```
**Fix:** Initialize submodules. If they're on flaky hosts (savannah), redirect first:
```yaml
- runs: git submodule set-url SUBMODULE https://MIRROR_URL
- runs: git submodule update --init
```

#### Missing build-time tool
```
make: TOOL: No such file or directory
configure: error: cannot find TOOL
```
**Fix:** Add the tool to `environment.contents.packages`. Common additions when switching from tarball:
- `libtool` — very common for autoconf projects
- `gettext-dev` — for projects with i18n
- `pkgconf-dev` — for pkg-config macros in bootstrap
- `texinfo` — for projects that build info pages
- `flex` / `bison` — for projects that generate parsers
- `help2man` — for projects that generate man pages at build time
- `wget` or `rsync` — for gnulib bootstrap downloading translations

#### Python SCM versioning error
```
LookupError: setuptools-scm was unable to detect version
```
**Fix:** Add `py3-supported-hatch-vcs` (or `py3-supported-setuptools-scm`) to build deps, which handles SCM-based version detection from the git checkout.

#### Tag or commit mismatch
```
error: expected commit X but got Y
```
The `expected-commit` is wrong. Re-resolve it:
```bash
git ls-remote --tags REPO "refs/tags/TAG^{}" "refs/tags/TAG" | awk '{print $1}' | head -1
```
Update `expected-commit` in the YAML.

#### `npm ci` fails — missing `package-lock.json`
```
npm error The `npm ci` command can only install with an existing package-lock.json
```
The release tarball bundles `package-lock.json`; the git repo does not commit it.
**Decision:** Cannot trivially convert. Leave on `fetch`. Example: `llhttp`.

#### Pre-generated build artifact missing
```
make: *** No rule to make target 'Makefile.sharedlibrary'
```
The release tarball includes pre-generated files (Makefiles, generated C sources, etc.) that are not in the git repo and are not regenerated by any available build step.
**Decision:** Cannot trivially convert. Leave on `fetch`. Example: `duktape`.

#### Release tarball includes pre-generated documentation; git checkout does not
```
This package [PKG-doc] is completely empty (i.e. installs no files).
```
Some projects ship pre-built man pages or HTML docs in their release tarballs but only generate them via optional tooling (sphinx, doxygen, etc.) not present in the build environment. The `test/docs` check then fails because the doc subpackage is empty.

**Fix options (in order of preference):**
1. Add the doc generator (sphinx, doxygen, `help2man`, etc.) to `environment.contents.packages` and a generation step to the pipeline.
2. If the docs were always empty even from the tarball (pre-existing bug), replace `test/docs` with `test/tw/emptypackage`.
3. If fixing is too complex, leave on `fetch`. Example: `gdal` (sphinx needed for man pages).

#### `ci-cve-scan-db-os` fails — pre-existing transitive CVE surfaces

When you modify a package (even just to convert fetch→git-checkout), the CI CVE scanner runs on the newly built APKs. It may find CVEs that **pre-existed** the conversion but were never detected by CI because the package hadn't been modified in a while.

Common case for Rust packages: a transitive dependency (e.g. `ratatui → lru 0.12.5`) has a known CVE. The existing yaml may already try to fix it with a `sed` that doesn't match the actual Cargo.toml format.

**Diagnosis:** Check if the CVE was already present on `origin/main` (look at the CGA advisory timestamp — if it was set weeks/months ago, it's pre-existing).

**Fix options:**
1. If the CVE is in a direct dependency: fix the `sed` pattern and verify with `cargo tree -i CRATE`.
2. If the CVE is in a **transitive** dependency (e.g. `ratatui→lru`): upgrading requires bumping the intermediate crate, which is a separate PR.
3. **Short-term:** revert the package from this PR; address the CVE in a dedicated PR. Example: `nushell` (ratatui 0.29.0 pulls in lru 0.12.5, GHSA-rhfx-m35p-ff5j).

### Common Test Failures After Conversion

Test failures after a fetch→git-checkout conversion are usually build failures in disguise (wrong binary, missing file). Re-check the build output for errors.

If the test itself fails:
1. Run `make test-debug/PACKAGE-NAME` to get an interactive shell inside the test environment
2. Manually run the failing commands to see the actual error
3. Fix the underlying cause in the YAML — never suppress failures with `|| true`

### Iterating on Failures

Build failures require editing the YAML and rebuilding. The cycle is:
1. Edit YAML
2. `(cd os && ./lint.sh PACKAGE.yaml)`
3. `make package/PACKAGE-NAME`
4. If build passes: `make test/PACKAGE-NAME`
5. If anything fails: diagnose from output, go to step 1

Do not proceed to commit until both `make package/` and `make test/` exit 0.

---

## Step 10: Commit

One commit per logical batch (single package or related group):

```
PKG: switch to git-checkout
```
or for multiple:
```
py3-d*: switch to git-checkout
```

**Never include "Co-Authored-By" lines.**

---

## Reference: cjwatson's Actual Conversion Examples

### Simple GitHub project (htop)
- fetch URI: `https://github.com/htop-dev/htop/releases/download/${{package.version}}/htop-${{package.version}}.tar.xz`
- git-checkout tag: `${{package.version}}` (no `v` prefix — tag on GitHub is bare version)
- update: `github: identifier: htop-dev/htop` (no strip-prefix, tag is bare version)

### GitHub project with `v` prefix (linux-pam)
- fetch URI: `https://github.com/linux-pam/linux-pam/releases/download/v${{package.version}}/Linux-PAM-${{package.version}}.tar.xz`
- git-checkout tag: `v${{package.version}}`
- update: `github: identifier: linux-pam/linux-pam` + `strip-prefix: v`

### GitHub project, bare version (harfbuzz)
- fetch URI: `https://github.com/harfbuzz/harfbuzz/releases/download/${{package.version}}/harfbuzz-${{package.version}}.tar.xz`
- git-checkout tag: `${{package.version}}`
- update: `github: identifier: harfbuzz/harfbuzz` (no strip-prefix — existing entry already had github: but had spurious `strip-prefix: v` which was **removed**)

### Custom tag prefix (erlang)
- fetch URI: `https://github.com/erlang/otp/releases/download/OTP-${{package.version}}/otp_src_${{package.version}}.tar.gz`
- git-checkout tag: `OTP-${{package.version}}`
- update: `github: identifier: erlang/otp` + `strip-prefix: OTP-` + `tag-filter-prefix: OTP-`

### PyPI → GitHub (py3-dnspython)
- fetch URI: `https://files.pythonhosted.org/packages/source/d/dnspython/dnspython-${{package.version}}.tar.gz`
- Found GitHub repo: `https://github.com/rthalley/dnspython`
- git-checkout tag: `v${{package.version}}`
- update: switched from `release-monitor: identifier: 13190` to `github: identifier: rthalley/dnspython` + strip-prefix: v

### PyPI → GitHub with extra build dep (py3-aiofiles)
- Added `py3-supported-hatch-vcs` to build environment (needed for SCM versioning)

### GNU autoconf with gnulib (sed, bison, cpio, findutils)
- Repository: `https://git.savannah.gnu.org/git/sed.git`
- tag: `v${{package.version}}`
- Added after git-checkout: gnulib URL redirect, `.tarball-version`, `./bootstrap`
- Added build deps: autoconf, automake, gettext-dev, texinfo, wget
- Keep `release-monitor:` (not GitHub)

### GitLab, autoconf with gnulib via GNULIB_URL (man-db, libpipeline)
- Repository: `https://gitlab.com/man-db/man-db`
- tag: `${{package.version}}`
- Added: `GNULIB_URL=https://github.com/coreutils/gnulib.git ./bootstrap`
- Added build deps: flex, libtool, pkgconf-dev, python3, wget

### Codeberg, re-enabled auto-update (chrpath)
- Was `enabled: false` with exclude-reason about no SCM
- Repository: `https://codeberg.org/pere/chrpath`, tag: `release-${{package.version}}`
- Re-enabled update, kept `release-monitor:` (not GitHub)
- Added `./bootstrap` step and `automake` dep

### Sparse checkout for large monorepo (font-opensymbol)
- Only needed `extras/source/truetype/symbol/` from the LibreOffice monorepo
- Used `sparse-paths:` parameter in git-checkout

### PECL extension (php-8.x-pecl-http)
- fetch URI: `https://pecl.php.net/get/pecl_http-${{package.version}}.tgz`
- Found GitHub: `https://github.com/m6w6/ext-http`
- tag: `v${{package.version}}`

---

## Handling Edge Cases

### Tag doesn't exist on remote
Some packages tag differently. If `v${{package.version}}` returns nothing:
```bash
git ls-remote --tags REPO_URL | grep "$VERSION" | head -20
```
Look at the pattern and adjust the tag template.

Some packages have no tags at all, or lack a tag for the specific version being packaged. In that case the conversion is not straightforward — file an upstream issue asking them to add tags, and skip the package for now.

### Version in tag uses different format
Example: erlang uses dots in version but tag is `OTP-25.3.2.21`. Construct the tag accordingly.

### Package has `extract: false` in fetch
The package was not a standard tarball (e.g., a `.deb` file). The git-checkout approach may require significant build process changes. Look at the full pipeline to understand what needs to change.

### Multiple packages sharing the same repo (version streams)
Convert all streams in one batch commit. They all point to the same `repository:` URL but different `tag:` values (different version numbers).

### Package is a subdirectory of a larger repository
Some packages only use a subdirectory of the upstream repo (e.g., a library nested inside a monorepo). These require adding `working-directory:` to the `git-checkout` step or other workarounds to point the build at the right path. These cases are usually complex — inspect the existing `fetch` URI and build pipeline carefully before attempting the conversion.

### Already has `github:` in update section
Some packages already have `github:` in their `update:` block but still use `fetch`. Only the pipeline needs updating; the `update:` section may only need minor adjustments (e.g., removing a `strip-prefix: v` if the tag turns out to be bare).

### Non-GitHub host → use `git:` update block
For savannah, gitlab.gnome.org, gitlab.freedesktop.org, codeberg.org etc.:
- Do NOT keep `release-monitor:` — the linter will reject it when `git-checkout` is used
- Switch to `git:` update block (the linter accepts this)
- If `enabled: false` cited "not in an SCM", re-enable it

---

## Quick-Check Script

After making changes, verify the conversion looks correct:

```bash
# Verify the tag resolves to a commit
REPO=$(grep "repository:" FILE.yaml | awk '{print $2}')
TAG=$(grep "tag:" FILE.yaml | awk '{print $2}')
VERSION=$(grep "^  version:" FILE.yaml | awk '{print $2}' | tr -d '"')
TAG_RESOLVED=${TAG/\$\{\{package.version\}\}/$VERSION}

echo "Resolving $REPO tag $TAG_RESOLVED"
git ls-remote --tags "$REPO" "refs/tags/${TAG_RESOLVED}^{}" "refs/tags/${TAG_RESOLVED}" 2>/dev/null
```
