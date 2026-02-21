---
name: rust-package
description: Create melange packages for Rust projects. Use when packaging Rust binaries, CLI tools, daemons, or services built with cargo.
---

# Rust Package Creation Skill

Follow CLAUDE.md "Package Test Best Practices" and "Packaging Best Practices" sections. This skill covers Rust-specific workflow.

## 1. Investigate Before Writing YAML

### Identify Project Type
Check the upstream repository to determine what the project produces:

1. **Check Cargo.toml** for `[[bin]]` sections - shows what binaries are built
2. **Check for Docker image** - if upstream publishes one, you'll likely need a `-compat` subpackage
3. **Check if it's a service** - look for config file examples, systemd units, or API documentation

| Type | What to Look For | Subpackages Needed |
|------|------------------|-------------------|
| **CLI Tool** | Single `[[bin]]` in Cargo.toml, no daemon mode | `-compat` (if Docker image exists) |
| **Daemon/Service** | Config files, `server` subcommand, listens on ports | `-compat`, test with `daemon-check-output` |
| **Multi-binary** | Multiple `[[bin]]` entries or workspace with multiple crates | Split binaries into subpackages |

### Check Build Requirements
1. **Cargo.toml**: Check for native dependencies (openssl, sqlite, etc.)
2. **build.rs**: Look for C library builds or protobuf generation
3. **Features**: Identify optional features that may be needed
4. **FIPS compatibility**: After build, run `rust-audit-info` on binary - if `ring`, `aws-lc-rs`, or `openssl` crates are present, FIPS is blocked

### Common Build Dependencies
| Dependency Type | Build Packages Needed |
|-----------------|----------------------|
| Basic Rust | `build-base`, `busybox`, `cargo-auditable`, `rust` |
| OpenSSL/TLS | Add `openssl-dev` |
| SQLite | Add `sqlite-dev` |
| Protobuf | Add `protoc` |
| Specific Rust version | Default to latest, use older version stream if not compatible |

**IMPORTANT: OpenSSL must not be vendored.** When a Rust package uses `openssl-dev` at build time, verify the binary dynamically links to system OpenSSL. Vendored/statically linked OpenSSL won't receive CVE fixes from system updates. See the OpenSSL dynamic linking test pattern in Section 4.

### Check Upstream Release Build
Official releases often enable non-default features. Look for `--features` in:
- **Nix**: `*.nix`, `flake.nix` (look for `cargo build` or `buildRustPackage`)
- **Docker**: `Dockerfile*`
- **CI**: `.github/workflows/release*.yml`, `.gitlab-ci.yml`

Match any non-default features in your `cargo/build` step.

### License
- Check `LICENSE`, `COPYING`, or `README` files
- Must be Open-Source license: https://opensource.org/licenses

## 2. Templates

### Standard Rust Binary

```yaml
package:
  name: project-name
  version: "X.Y.Z"
  epoch: 0
  description: Short description
  copyright:
    - license: MIT

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  - uses: rust/cargobump

  - uses: cargo/build
    with:
      output: project-name

subpackages:
  # If the original upsteam container image places the binary in an alternative
  # location, create a compat package. Requires inspecting upstream image.
  - name: ${{package.name}}-compat
    description: Compat package for upstream Docker image
    pipeline:
      - runs: |
          mkdir -p "${{targets.contextdir}}"
          ln -sf /usr/bin/project-name "${{targets.contextdir}}/project-name"
    test:
      environment:
        contents:
          packages:
            - ${{package.name}}
      pipeline:
        - uses: test/tw/symlink-check
          with:
            allow-absolute: true

  # Documentation subpackage (if project generates man pages)
  - name: ${{package.name}}-doc
    description: Documentation for ${{package.name}}
    pipeline:
      - runs: |
          mkdir -p "${{targets.contextdir}}/usr/share/man"
          target/release/project-name util install-man-pages "${{targets.contextdir}}/usr/share/man"
      - uses: split/manpages
    test:
      pipeline:
        - uses: test/docs

update:
  enabled: true
  ignore-regex-patterns:
    - '-rc'
    - '-beta'
  github:
    identifier: org/project
    use-tag: true
    strip-prefix: v

test:
  pipeline:
    - uses: test/tw/ldd-check
    - uses: test/tw/help-check
      with:
        bins: project-name
    - uses: test/tw/ver-check
      with:
        bins: project-name
    # REQUIRED: Add functional tests - see Section 4
```

### With Feature Flags

```yaml
pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  - uses: rust/cargobump

  - uses: cargo/build
    with:
      output: project-name
      features: feature1,feature2
```

### With Custom RUSTFLAGS

```yaml
pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  - uses: rust/cargobump

  - uses: cargo/build
    with:
      output: project-name
      rustflags: "-C linker=/usr/local/bin/gcc"
```

## 3. CVE Remediation

### Understanding Rust Dependency Trees
Unlike Go, Rust can have **multiple versions of the same crate** in the dependency tree. Before bumping, you must identify which version is vulnerable.

### Step 1: Investigate the Dependency Tree
Use `cargo tree -i` to see what depends on a vulnerable crate:

```bash
# Show inverse dependency tree - what pulls in this crate
cargo tree -i vulnerable-crate

# If multiple versions exist, you'll see them listed
cargo tree -i hashbrown
```

This shows which path in the tree has the vulnerable version.

### Step 2: Use rust/cargobump (Primary Method)
Create `project-name/cargobump-deps.yaml`:

```yaml
packages:
  - name: vulnerable-crate
    version: 1.2.3  # Fixed version - CVE-YYYY-XXXXX
  - name: another-crate
    version: 4.5.6
```

The pipeline runs `cargo update --precise <version> --package <name>@<oldVersion>` for each entry:

```yaml
pipeline:
  - uses: git-checkout
    # ...

  - uses: rust/cargobump  # Reads ./cargobump-deps.yaml if present

  - uses: cargo/build
    with:
      output: project-name
```

### Step 3: Fallback - Manual cargo update
If `rust/cargobump` doesn't work (e.g., can't find the old version), use `cargo update -p` directly:

```yaml
pipeline:
  - uses: git-checkout
    # ...

  - runs: |
      # Target specific version when multiple exist in tree
      cargo update -p hashbrown@0.15.0 --precise 0.15.1
      cargo update -p trust-dns-proto --precise 0.23.2

  - uses: cargo/build
    with:
      output: project-name
```

The `@oldVersion` syntax is key when multiple versions of the same crate exist in the tree.

### Verify the Fix
After bumping, verify the vulnerable version is gone:

```bash
cargo tree -i vulnerable-crate
# Should show only the fixed version
```

## 4. Functional Test Coverage

**Tests must validate real functionality, not just "does it compile" or "does --help work".**

Version checks (`--version`) and help checks (`--help`) are NOT functional tests. Functional tests must exercise the binary doing its actual job.

### How to Identify What to Test

1. **Read the project description** - what does it claim to do?
2. **Check `--version` output** - what features are compiled in? Test them.
3. **Look at subcommands** - `tool help` shows available commands to test
4. **Identify the project type** - daemon, CLI tool, library? Each has different testing needs.
5. **Think about real use cases** - what would users actually do with this?

### Coverage Checklist

For each package, consider testing:

- [ ] **CLI tools** - test actual operations, not just --help/--version
- [ ] **Core functionality** - the main purpose of the program
- [ ] **Compiled features** - if --version shows "with ssl", test SSL works
- [ ] **Config types** - if it handles different modes/protocols, test each type
- [ ] **API/Interface** - if it has HTTP/REST/gRPC/socket interface, test endpoints
- [ ] **Input/Output** - test with real data, verify correct output

### By Project Type

| Type | Must Test |
|------|-----------|
| **CLI Tool** | Core operations with real input → verify output |
| **Daemon/Service** | Start service, test API/endpoints, verify responses |
| **Storage Service** | CRUD operations, data integrity, multipart uploads |
| **Proxy/Gateway** | Connection handling, routing, health checks |

### Example: S3-Compatible Storage (garage)
For an S3-compatible storage service, we tested:
1. **Daemon startup** - service starts, listens on expected ports
2. **Health endpoint** - admin API responds
3. **Key management** - create/list API keys
4. **Bucket operations** - create, list, configure permissions
5. **S3 operations** - upload, download, delete, list objects
6. **Multipart upload** - large file upload (>5MB)
7. **Server-side copy** - CopyObject API
8. **Presigned URLs** - temporary signed access
9. **Static website** - website hosting feature
10. **Metrics endpoint** - Prometheus metrics exposure

**Ask yourself: "If this test passes but the binary is broken, what did I miss?"**

### Using test/daemon-check-output
For services that need to run:

```yaml
test:
  pipeline:
    - uses: test/daemon-check-output
      with:
        setup: |
          mkdir -p /tmp/data
          tee /tmp/config.toml << 'EOF'
          # Configuration here
          EOF
        start: project-name --config /tmp/config.toml
        timeout: 30
        expected_output: "Server started"
        post: |
          set -euo pipefail
          # Test API endpoints
          curl -sf http://127.0.0.1:8080/health
          # More functional tests...
```

### Verifying OpenSSL Dynamic Linking
When `openssl-dev` is a build dependency, add a test to verify the binary dynamically links to system OpenSSL (not vendored):

```yaml
test:
  environment:
    contents:
      packages:
        - binutils  # provides readelf and nm
  pipeline:
    - name: verify dynamic linking of openssl
      runs: |
        readelf --dynamic /usr/bin/project-name | grep NEEDED | grep -E 'libssl|libcrypto'
        readelf --dyn-syms /usr/bin/project-name | grep OPENSSL
        nm --dynamic /usr/bin/project-name | grep 'U SSL_.*@OPENSSL_'
```

This ensures:
- The binary has NEEDED entries for libssl/libcrypto (dynamic dependencies)
- OPENSSL symbols are present in the dynamic symbol table
- SSL functions are undefined (U) and will be resolved from system OpenSSL at runtime

If these checks fail, the binary may have vendored OpenSSL and won't receive system CVE fixes.

## 5. Advanced Patterns

### Build Metadata Injection
Embed version/commit info at compile time via environment variables:

```yaml
pipeline:
  - runs: |
      export PROJECT_BUILD_STATUS=clean
      export PROJECT_BUILD_GIT_REVISION=$(git rev-parse HEAD)
      export PROJECT_BUILD_VERSION=$(git describe --tags --abbrev=0)

  - uses: cargo/build
    with:
      output: project-name
```

## 6. FIPS Package Creation

Creating a FIPS variant of a Rust package requires a multi-step verification process.

**CRITICAL: Do NOT stop at source code analysis. Do NOT make FIPS decisions based on cargo tree output alone.**

The only way to determine FIPS compatibility is to **build the package and scan the actual binary**.

### Step 1: Build the Package FIRST

Always start by building the non-FIPS package:

```bash
make package/project-name
```

This is required before any FIPS analysis. Source code analysis is optional and only provides hints.

### Step 2: Scan the Built Binary

After building, scan the actual binary for FIPS-blocking crates:

```bash
# Method 1: Use rust-audit-info (preferred - checks embedded dependency info)
rust-audit-info packages/aarch64/project-name-*.apk 2>&1 | grep -iE 'ring|aws-lc-rs|openssl|sodiumoxide|libsodium'

# Method 2: Check strings in binary (fallback)
# First extract the package
mkdir -p /tmp/scan && tar -xzf packages/aarch64/project-name-*.apk -C /tmp/scan
strings /tmp/scan/usr/bin/project-name | grep -iE 'ring|sodiumoxide|libsodium|aws-lc'

# Method 3: Check dynamic linking (verify no vendored OpenSSL)
# From melange build output, check "found lib" lines - should NOT show libssl/libcrypto
# unless the package is supposed to use system OpenSSL
```

### Step 3: Evaluate Binary Scan Results

| Binary Scan Result | FIPS Status | Action |
|--------------------|-------------|--------|
| No blocking crates found, uses system OpenSSL | ✅ FIPS OK | Create FIPS package with `openssl-config-fipshardened` |
| No blocking crates, no OpenSSL (pure Rust crypto like rustls) | ⚠️ Review needed | May not need FIPS variant if no sensitive crypto ops |
| `ring` found in binary | ❌ Blocked | Report blocker, do not create FIPS package |
| `aws-lc-rs` found in binary | ❌ Blocked | Report blocker, do not create FIPS package |
| `openssl` crate with vendored/static linking | ❌ Blocked | Report blocker, do not create FIPS package |
| `libsodium`/`sodiumoxide` found | ❌ Blocked | Report blocker - libsodium is not FIPS-validated |

### Step 4: Create FIPS Package (if not blocked)

If the binary passes scanning, create the FIPS variant:

```yaml
# project-name-fips.yaml
package:
  name: project-name-fips
  version: "X.Y.Z"
  epoch: 0
  description: FIPS-enabled project-name
  copyright:
    - license: MIT
  dependencies:
    runtime:
      - openssl-config-fipshardened
    provides:
      - project-name=${{package.full-version}}

# ... rest of package mirrors non-FIPS version
```

### FIPS Blocker Summary

| Crate/Library | Why It Blocks FIPS |
|---------------|-------------------|
| `ring` | Pure Rust crypto, not FIPS-validated |
| `aws-lc-rs` | AWS crypto library, complex FIPS story |
| `openssl` (vendored) | Bypasses system FIPS OpenSSL |
| `libsodium`/`sodiumoxide` | Not a FIPS-validated cryptographic module |
| `rustls` without system crypto | Uses ring or aws-lc-rs internally |

**Do not attempt to patch out these dependencies** - this requires upstream changes. Report the blocker and explain why FIPS is not possible.

### Optional: Source Code Analysis (for context only)

If you want to understand what crypto the project uses before building, you can check the dependency tree. **This is informational only - never make FIPS decisions based on this.**

```bash
# Clone the repo at the target version
git clone --depth 1 --branch vX.Y.Z https://github.com/org/project /tmp/project-check
cd /tmp/project-check

# Check for potential blockers in NON-DEV dependencies
# Use -e no-dev to exclude dev-dependencies (they don't end up in the binary)
cargo tree -e no-dev -i ring
cargo tree -e no-dev -i aws-lc-rs
cargo tree -e no-dev -i openssl
cargo tree -e no-dev | grep -iE "sodiumoxide|libsodium"
```

Even if this shows blockers, **still build and scan the binary** - source analysis can be misleading (feature flags, conditional compilation, etc.).

## 7. Gotchas and Special Handling

### Pinning Rust Versions
Some projects require specific Rust toolchain versions:

```yaml
environment:
  contents:
    packages:
      - rust-1.80  # Pin to specific version instead of latest
```

### The cargo/build Pipeline
The `cargo/build` pipeline automatically:
- Uses `cargo-auditable` for SBOM embedding
- Builds in release mode (`--release`) by default via the `opts` parameter
- Installs to the correct destination

When overriding `opts`, you MUST include `--release` explicitly — the default `opts` value includes `--release`, so overriding it without `--release` will produce a debug build. For example:

```yaml
  - uses: cargo/build
    with:
      output: project-name
      opts: --release --no-default-features
```

Always use `cargo/build` instead of raw `cargo build` commands.

## 8. Build and Test

After creating the YAML file, use the `build-test` skill:
1. Build the package: `make package/NAME`
2. Run tests: `make test/NAME`
3. Scan for CVEs: `wolfictl scan packages/aarch64/NAME-*.apk`
4. If CVEs found, create/update `cargobump-deps.yaml` and rebuild

## 9. Checklist

Before submitting:

**Build Configuration:**
- [ ] License verified from Cargo.toml and LICENSE file
- [ ] Correct commit hash (not annotated tag)
- [ ] Checked upstream release build for non-default features (Nix/Docker/CI)
- [ ] `rust/cargobump` pipeline step added
- [ ] `cargo/build` pipeline used (handles cargo-auditable automatically)
- [ ] Minimal build dependencies

**CVE Remediation:**
- [ ] Investigated dep tree with `cargo tree -i <crate>` before bumping
- [ ] `cargobump-deps.yaml` created if CVE fixes needed
- [ ] Scanned with `wolfictl scan` after build
- [ ] Verified fix with `cargo tree -i` after bump

**Subpackages:**
- [ ] `-compat` subpackage with symlink test (if Docker image exists)
- [ ] `-doc` subpackage with `test/docs` (if project generates man pages)

**Testing:**
- [ ] `test/tw/ldd-check` on main package
- [ ] `test/tw/help-check` and `test/tw/ver-check` for CLI tools
- [ ] Functional tests covering core features (not just --help/--version)
- [ ] OpenSSL dynamic linking verified (if `openssl-dev` is a build dependency)

**Update Configuration:**
- [ ] Update section with correct `tag-filter-prefix` (for versioned packages)
- [ ] `use-tag: true` if no GitHub releases (only tags)

**FIPS Package (if creating -fips variant):**
- [ ] Built non-FIPS package first
- [ ] Scanned binary with `rust-audit-info` or `strings` for blocking crates
- [ ] Verified no `ring`, `aws-lc-rs`, `libsodium`, or vendored `openssl` in binary
- [ ] Added `openssl-config-fipshardened` runtime dependency
- [ ] Added `provides: project-name=${{package.full-version}}`
