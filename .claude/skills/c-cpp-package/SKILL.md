---
name: c-cpp-package
description: Create melange packages for C/C++ projects. Use when packaging C/C++ libraries, CLI tools, daemons, or system utilities.
---

# C/C++ Package Creation Skill

Follow CLAUDE.md "Package Test Best Practices" and "Packaging Best Practices" sections. This skill covers C/C++ specific workflow.

## 1. Investigate Before Writing YAML

### Identify Build System
Check the repository for build system files:

| Files Present | Build System | Pipelines to Use |
|---------------|--------------|------------------|
| `CMakeLists.txt` | CMake | `cmake/configure`, `cmake/build`, `cmake/install` |
| `configure.ac`, `Makefile.am` | Autoconf | `autoconf/configure`, `autoconf/make`, `autoconf/make-install` |
| `meson.build` | Meson | `meson/configure`, `meson/compile`, `meson/install` |
| `Makefile` only | Custom make | Direct `runs:` with make commands |

**Pre-configure steps:**
- Some autoconf projects need `./autogen.sh` before configure
- Check for `autogen.sh`, `bootstrap.sh`, or similar scripts

### Identify Project Type
| Type | Characteristics | Subpackages Needed |
|------|-----------------|-------------------|
| **Library** | Installs `.so`, `.a`, headers | `-dev`, `-libs`, `-static` |
| **CLI Tool** | Installs binaries to `/usr/bin` | `-doc` (if has man pages) |
| **Daemon** | Long-running service | `-doc`, possibly `-compat` |
| **Header-only** | Only `.h` files | `-dev` only |

### Check Dependencies
1. **Build dependencies**: Check `CMakeLists.txt`, `configure.ac`, or `meson.build` for required libraries
2. **Runtime dependencies**: Usually minimal - melange auto-detects `.so` dependencies
3. **Add `-dev` packages**: For each library dependency at build time

### License
- Check `LICENSE`, `COPYING`, or `README` files
- Must be Open-Source license: https://opensource.org/licenses

## 2. Templates

### CMake Project

```yaml
package:
  name: project-name
  version: "X.Y.Z"
  epoch: 0
  description: Short description
  copyright:
    - license: MIT

environment:
  contents:
    packages:
      - build-base
      - busybox
      - cmake
      # Add *-dev packages for library dependencies

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  - uses: cmake/configure
    with:
      opts: |
        -DOPTION=value

  - uses: cmake/build

  - uses: cmake/install

  - uses: strip

subpackages:
  - name: ${{package.name}}-dev
    pipeline:
      - uses: split/dev
    test:
      pipeline:
        - uses: test/pkgconf  # Only if has .pc files
        - uses: test/tw/ldd-check

  - name: ${{package.name}}-doc
    pipeline:
      - uses: split/alldocs
    test:
      pipeline:
        - uses: test/docs

update:
  enabled: true
  github:
    identifier: org/project
    strip-prefix: v

test:
  pipeline:
    - uses: test/tw/ldd-check
    # REQUIRED: Add functional tests per Section 4 coverage guidance (see Section 5 for examples)
```

### Autoconf Project

```yaml
environment:
  contents:
    packages:
      - build-base
      - busybox
      - libtool  # Often needed for autoconf
      # Add *-dev packages for library dependencies

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  # If project needs autogen.sh before configure:
  - runs: ./autogen.sh
  # Or: runs: env NOCONFIGURE=1 ./autogen.sh

  - uses: autoconf/configure
    with:
      opts: |
        --disable-static \
        --enable-feature

  - uses: autoconf/make

  - uses: autoconf/make-install

  - uses: strip
```

### Meson Project

```yaml
environment:
  contents:
    packages:
      - build-base
      - busybox
      - cmake  # Meson often needs cmake
      - meson
      # Add *-dev packages for library dependencies

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: ${{package.version}}
      expected-commit: COMMIT_HASH

  - uses: meson/configure
    with:
      opts: |
        -Doption=value

  - uses: meson/compile

  - uses: meson/install

  - uses: strip
```

### Custom Make Project

```yaml
pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: ${{package.version}}
      expected-commit: COMMIT_HASH

  - runs: |
      make -j$(nproc)
      make install PREFIX=/usr DESTDIR=${{targets.destdir}}

  - uses: strip
```

## 3. Subpackage Patterns

### Standard Library Subpackages

```yaml
subpackages:
  # Development files (headers, .pc, .a, .so symlinks)
  - name: ${{package.name}}-dev
    description: Development files for ${{package.name}}
    pipeline:
      - uses: split/dev
    dependencies:
      runtime:
        - ${{package.name}}  # Dev package depends on main package
    test:
      pipeline:
        - uses: test/pkgconf  # Only if package has .pc files
        - uses: test/tw/ldd-check

  # Shared libraries (lib*.so.*)
  - name: ${{package.name}}-libs
    description: Shared libraries for ${{package.name}}
    pipeline:
      - uses: split/lib
    test:
      pipeline:
        - uses: test/tw/ldd-check

  # Static libraries
  - name: ${{package.name}}-static
    description: Static libraries for ${{package.name}}
    pipeline:
      - uses: split/static

  # Documentation/man pages
  - name: ${{package.name}}-doc
    description: Documentation for ${{package.name}}
    pipeline:
      - uses: split/alldocs
    test:
      pipeline:
        - uses: test/docs
```

## 4. Functional Test Coverage

**Tests must validate real functionality, not just "does it compile" or "does config parse".**

Config syntax validation (`tool -t config`) is NOT a functional test. Functional tests must exercise the binary doing its actual job.

### How to Identify What to Test

1. **Read the project description** - what does it claim to do?
2. **Check `--version` output** - what features are compiled in? Test them.
3. **Look at the main commands/subcommands** - what operations does it provide?
4. **Identify the project type** - daemon, CLI tool, library? Each has different testing needs.
5. **Think about real use cases** - what would users actually do with this?

### Coverage Checklist

For each package, consider testing:

- [ ] **CLI tools** - test actual operations, not just --help/--version
- [ ] **Core functionality** - the main purpose of the program
- [ ] **Compiled features** - if --version shows "with ssl", test SSL works
- [ ] **Config types** - if it monitors files/processes/network, test each type
- [ ] **API/Interface** - if it has HTTP/REST/socket interface, test endpoints
- [ ] **Input/Output** - test with real data, verify correct output

### By Project Type

| Type | Must Test |
|------|-----------|
| **CLI Tool** | Core operations with real input → verify output |
| **Daemon** | Start service, test API/endpoints, verify responses |
| **Library** | Compile test program that calls main API functions |
| **Parser** | Parse real data, verify parsed output |
| **Monitor** | Test each monitoring type (file, process, network, etc.) |

### Example: monit Coverage

Monit is a process/file/system monitor with HTTP interface. We tested:

1. **Compiled features** - verified ssl, pam, compression in --version output
2. **Hash computation** - tested -H flag with real file, verified SHA1/MD5 output
3. **Process matching** - tested procmatch command finds processes
4. **File monitoring** - config with checksum, permissions, size, timestamp checks
5. **Directory monitoring** - config with permission and timestamp checks
6. **Filesystem monitoring** - config with space and inode usage checks
7. **Program checks** - config that runs external program and checks exit status
8. **Network monitoring** - config with ping and port/protocol checks
9. **Process monitoring** - config with pidfile and pattern matching
10. **HTTP interface** - started daemon, tested web UI, status endpoint, XML output

**Ask yourself: "If this test passes but the binary is broken, what did I miss?"**


## 5. Checklist

Before submitting:

- [ ] Source via `git-checkout` (not pre-built tarball)
- [ ] Build system correctly identified (cmake/autoconf/meson/make)
- [ ] License verified from upstream
- [ ] Correct commit hash (not annotated tag)
- [ ] Minimal build dependencies (only what's needed)
- [ ] Runtime dependencies minimal (melange auto-detects .so deps)
- [ ] `-dev` subpackage with `test/pkgconf` (if has .pc files) and `test/tw/ldd-check`
- [ ] `-doc` subpackage with `split/alldocs` and `test/docs` (if has man pages or info docs)
- [ ] `-libs` subpackage with `split/lib` if shipping shared libraries separately
- [ ] Functional tests per Section 4 (not just --help/--version or config syntax checks)
- [ ] `test/tw/ldd-check` on main package
- [ ] Update section configured correctly

## 8. Build and Test

After creating the YAML file, use the `build-test` skill:
1. Build dependencies first (if any)
2. Build the package: `make package/NAME`
3. Run tests: `make test/NAME`
