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
