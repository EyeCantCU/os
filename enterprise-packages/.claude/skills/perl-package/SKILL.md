---
name: perl-package
description: Create melange packages for Perl CPAN modules. Use when packaging Perl modules, creating perl-* packages, or working with CPAN distributions.
---

# Perl Package Creation Skill

Follow testing and packaging best practices in CLAUDE.md. This skill covers Perl-specific workflow.

## 1. Investigate Before Writing YAML

### Source Repository
1. Check if module has GitHub/GitLab repo with tags
2. Use `git-checkout` if tags exist (preferred)
3. Use `fetch` from CPAN only if no git tags (add comment explaining why)

### Update Tracking
1. **Check GitHub Releases first**: `https://github.com/AUTHOR/Module-Name/releases`
   - If releases exist: don't add `use-tag: true`
2. **If no releases, check tags**: `https://github.com/AUTHOR/Module-Name/tags`
   - If only tags exist: add `use-tag: true`

### Release Monitoring ID
**CRITICAL: Always verify the release-monitoring.org project ID**

1. **Search release-monitoring.org**:
   Search for the module name (without perl- prefix)
   Here's a general example that can be applied for future packages: For `perl-devel-globaldestruction`, search for "Devel-GlobalDestruction"
   Visit: https://release-monitoring.org/projects/search/?pattern=Devel-GlobalDestruction

2. **Verify the correct project**:
   - Click on the project to see its page
   - Verify the homepage URL matches the CPAN module
   - Note the project ID from the URL, for example: `https://release-monitoring.org/project/2832/`
   - Use `2832` as the identifier

3. **Common mistakes**:
   - ✗ Using a similar but wrong project ID
   - ✗ Guessing the ID without verification
   - ✓ Always search and verify on release-monitoring.org

### Version Mangling
Some Perl projects use non-standard tag formats (e.g., `5_013` instead of `5.013`). Use `var-transforms` to convert the version for git-checkout, and `version-transform` in update section for the reverse:

```yaml
var-transforms:
  - from: ${{package.version}}
    match: \.(\d+)$
    replace: _$1
    to: mangled-package-version

pipeline:
  - uses: git-checkout
    with:
      tag: ${{vars.mangled-package-version}}

update:
  version-transform:
    - match: "_"
      replace: "."
```

### Module Type
- **Pure Perl**: Only needs `busybox`, `perl` at build time
- **XS Module**: Needs `build-base`, `perl-dev`, plus library deps

### Dependencies
**CRITICAL: Always verify dependencies exist before adding them to the YAML.**

#### 1. Identify Dependencies
Fetch and examine the module's `Makefile.PL` or `META.json` from upstream:
```bash
# Check dependencies in Makefile.PL
curl -s https://raw.githubusercontent.com/AUTHOR/Module-Name/TAG/Makefile.PL | grep -A 20 PREREQ_PM

# Or check META.json
curl -s https://raw.githubusercontent.com/AUTHOR/Module-Name/TAG/META.json | jq '.prereqs.runtime.requires'
```

**Look for runtime dependencies only** - exclude test-only deps like `Test::More`, `Test::Fatal`.

#### 2. Convert Module Names to Package Names
**Convention:** `Module::Name` → `perl-module-name` (lowercase, `::` becomes `-`)

Examples:
- `Devel::GlobalDestruction` → `perl-devel-globaldestruction`
- `Params::ValidationCompiler` → `perl-params-validationcompiler`
- `Try::Tiny` → `perl-try-tiny`

#### 3. Filter Out Core Perl Modules
**Do not add core modules to dependencies** (they ship with the `perl` package):
- Common core: `Carp`, `Exporter`, `Fcntl`, `IO::Handle`, `Scalar::Util`, `base`, `parent`, `strict`, `warnings`
- Often core: `Encode`, `Sys::Syslog`, `File::Spec`, `File::Path`
- When unsure, check: https://perldoc.perl.org/modules or `corelist Module::Name`

#### 4. Verify Package Availability
**Check packages exist in this order:**

**Step 1: Check local enterprise-packages repo first**
```bash
# From repo root - check if package YAML exists locally
ls -1 perl-*.yaml | grep "perl-devel-globaldestruction"

# Or list all perl packages
find . -maxdepth 1 -name "perl-*.yaml" -exec basename {} .yaml \;
```

**Step 2: If not local, check published repos**
```bash
check_package() {
  local pkg=$1
  echo "Checking: $pkg"

  # Check key Wolfi/Chainguard repos
  local repos=(
    "https://packages.wolfi.dev/os/x86_64"
    "https://packages.cgr.dev/extras/x86_64"
    "https://apk.cgr.dev/chainguard/x86_64"
  )

  for repo in "${repos[@]}"; do
    if curl -sf "$repo/APKINDEX.tar.gz" | tar -xzO APKINDEX | grep -q "^P:$pkg$"; then
      echo "  ✓ Found in: $repo"
      return 0
    fi
  done

  echo "  ✗ NOT FOUND"
  return 1
}

# Usage
check_package perl-specio
check_package perl-devel-globaldestruction
```

#### 5. Handle Missing Dependencies
**If a package doesn't exist:**
- ✗ **DON'T** add it to dependencies (causes: `ERROR: nothing provides "package-name"`)
- ✓ **DO** mark dependency with FIXME in a comment
- ✓ **DO** create missing packages first, build it, then remove the FIXME label and uncomment

**Example YAML with missing dependencies:**
```yaml
dependencies:
  runtime:
    - perl-module-runtime  # ✓ exists in wolfi/os
    - perl-specio          # ✓ exists in wolfi/os
    # MISSING - need to be packaged first:
    # - perl-devel-globaldestruction
    # - perl-params-validationcompiler
```

#### 6. Dependency Build Order
When packages depend on each other:
1. Build leaf dependencies first (no perl-* runtime deps)
2. Build packages that depend on them next
3. Test each before proceeding

**Example build order for perl-log-dispatch:**
```
perl-devel-globaldestruction (no deps) → build first
  └─→ perl-namespace-autoclean (needs above) → build second
       └─→ perl-log-dispatch (needs above) → build last
```

#### 7. Discovering Transitive Dependencies
**Makefile.PL only lists direct dependencies, not transitive ones.**

Transitive dependencies are discovered through testing:

**Workflow:**
1. Build package with direct dependencies only
2. Run tests: `make test/PACKAGE`
3. If test fails with `Can't locate Module.pm`, that's a missing transitive dependency
4. Convert module name to package name, verify it exists
5. Add to runtime dependencies
6. Repeat until tests pass

**Example test failure:**
```
Can't locate Clone.pm in @INC (you may need to install the Clone module)
```

**Resolution:**
```bash
# 1. Convert to package name: Clone → perl-clone
# 2. Verify it exists
curl -sf "https://packages.wolfi.dev/os/x86_64/APKINDEX.tar.gz" | \
  tar -xzO APKINDEX | grep -A 2 "^P:perl-clone$"

# 3. Add to dependencies (alphabetically)
dependencies:
  runtime:
    - perl-clone  # Added after test failure
    - perl-other-dep
```

**Common transitive dependency patterns:**
- `perl-specio` → needs `perl-clone`
- `perl-moose` → needs `perl-package-stash`, `perl-class-load`
- Database modules → need driver-specific packages

**Note:** This is iterative - you may need multiple test-fix cycles to discover all transitive dependencies.

### License
- Check LICENSE file or META.json
- Most use "same terms as Perl itself" → `GPL-1.0-or-later OR Artistic-1.0-Perl`
- Always verify from upstream, never assume

### CLI Tools
- Check `bin/` or `script/` directories for installed commands
- Must be tested

## 2. Templates

### Pure Perl (git tags)

```yaml
package:
  name: perl-module-name
  version: "X.XX"
  epoch: 0
  description: Short description from CPAN
  copyright:
    - license: GPL-1.0-or-later OR Artistic-1.0-Perl

environment:
  contents:
    packages:
      - busybox
      - perl

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/AUTHOR/Module-Name
      tag: vX.XX
      expected-commit: COMMIT_HASH

  - uses: perl/make

  - uses: autoconf/make

  - uses: autoconf/make-install

  - uses: perl/cleanup

subpackages:
  - name: ${{package.name}}-doc
    pipeline:
      - uses: split/alldocs
    description: Documentation for ${{package.name}}
    test:
      pipeline:
        - uses: test/docs

update:
  enabled: true
  github:
    identifier: AUTHOR/Module-Name
    strip-prefix: v
    # use-tag: true  # Only if no GitHub releases

test:
  # No need to specify runtime dependencies in test environment, which are already
  # defined as runtime packages by the primary package - they're automatic.
  pipeline:
    # See "Perl Test Examples" below
```

### CPAN-only (no git tags)

```yaml
pipeline:
  # Use fetch when upstream has no git tags for releases
  # CPAN is the source of truth for this module's versioning
  - uses: fetch
    with:
      uri: https://cpan.metacpan.org/authors/id/A/AU/AUTHOR/Module-Name-${{package.version}}.tar.gz
      expected-sha256: SHA256_HASH

  - runs: tar -xzf Module-Name-${{package.version}}.tar.gz --strip-components=1

  - uses: perl/make

  # ... rest same as above
```

### XS Module (compiled)

```yaml
environment:
  contents:
    packages:
      - build-base
      - busybox
      - perl
      - perl-dev
      # Add library deps: openssl-dev, mysql-dev, etc.

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/AUTHOR/Module-Name
      tag: ${{package.version}}
      expected-commit: COMMIT_HASH

  - runs: PERL_MM_USE_DEFAULT=1 perl Makefile.PL INSTALLDIRS=vendor

  - uses: autoconf/make

  - uses: autoconf/make-install

  - runs: find "${{targets.destdir}}" \( -name perllocal.pod -o -name .packlist \) -delete

  - uses: strip

test:
  # No need to specify runtime dependencies in test environment, which are already
  # defined as runtime packages by the primary package - they're automatic.
  pipeline:
    - runs: perl -e 'use Module::Name; print "loaded\n"'
    - uses: test/tw/ldd-check  # Required for XS modules
```

## 3. Functional Test Coverage

**Tests must validate real functionality, not just "does it load".**

### How to Identify What to Test

1. **Read the module's SYNOPSIS/description** - what does it claim to do?
2. **Check upstream tests** - what do maintainers test?
3. **Look at the main exports** - what functions/methods does it provide?
4. **Think about real use cases** - what would users actually do with this module?

### Coverage Checklist

For each module, consider testing:
- [ ] **CLI tools** - if any are installed, test their actual functionality
- [ ] **Core functions** - the main purpose of the module
- [ ] **Input variations** - different formats, edge cases
- [ ] **Output formatting** - if module formats/transforms data
- [ ] **Integration features** - timezone support, language support, etc.

### Example: perl-date-manip Coverage

This module handles dates, so we tested 9 aspects:
1. CLI tools (dm_date, dm_zdump) - with correct syntax
2. Module loading - main module and submodules
3. Date parsing - ISO format, text format, relative ("today", "next friday")
4. Date arithmetic - adding months, calculating deltas
5. Date formatting - various output formats
6. Timezone support - parsing and converting timezones
7. Date comparison - comparing two dates
8. Multi-language - German, French, Spanish date parsing

**Ask yourself: "If this test passes but the module is broken, what did I miss?"**

## 4. Checklist

- [ ] License verified from upstream
- [ ] Correct commit hash (not annotated tag)
- [ ] GitHub releases vs tags checked for update config
- [ ] Doc subpackage with test/docs
- [ ] Functional tests covering core features (not just --help)
- [ ] CLI tools tested with correct syntax
- [ ] XS modules have test/tw/ldd-check
- [ ] Minimal build dependencies

## 5. Build and Test

After creating the YAML file, use the `build-test` skill to build and verify the package works:
1. Build dependencies first (if any)
2. Build the package: `make package/NAME`
3. Run tests: `make test/NAME`
