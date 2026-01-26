# FIPS Go Packages

This document covers FIPS-specific requirements for Go packages.

## go-fips vs go-fips-md5

| Toolchain | Provider | Use Case |
|-----------|----------|----------|
| `go-fips` | `go-msft-*` | Standard FIPS 140-3 approved algorithms only |
| `go-fips-md5` | `go-fips-*` | FIPS + non-cryptographic MD5 allowed |

**Use `go-fips-md5` when:**
- Project uses MD5 for checksums, content hashing, ETags
- Error messages mention "MD5" or "non-approved algorithm"
- Examples: istio, vault, skaffold, karpenter

## FIPS Package Template

```yaml
package:
  name: project-name-fips
  version: "X.Y.Z"
  epoch: 0
  description: FIPS-enabled project-name
  copyright:
    - license: Apache-2.0

# NOTE: No environment.contents.packages needed for pure go/build pipeline
# NOTE: Do NOT add `provides: project-name` - see "provides" section below

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  - uses: go/build
    with:
      packages: ./cmd/project
      output: project-name
      go-package: go-fips
      ldflags: -X main.version=${{package.version}}

update:
  enabled: true
  github:
    identifier: org/project
    strip-prefix: v

test:
  pipeline:
    - uses: test/go-fips-check
    - uses: test/tw/ldd-check
    - uses: test/tw/ver-check
      with:
        bins: project-name
    # REQUIRED: Add functional tests
```

## CNI Plugin Package (FIPS)

CNI plugins require special handling for glibc compatibility:

```yaml
package:
  name: cni-plugin-fips
  version: "X.Y.Z"
  epoch: 0
  description: FIPS-enabled CNI plugin
  copyright:
    - license: Apache-2.0

environment:
  contents:
    packages:
      # CRITICAL: Required for ABI compatibility with older host systems
      - oldglibc~2.28

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/cni-plugin
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  - uses: go/build
    with:
      packages: ./cmd/cni-plugin
      output: cni-plugin
      go-package: go-fips
      ldflags: -X main.version=${{package.version}}

subpackages:
  - name: ${{package.name}}-compat
    description: Compat package for CNI bin directory
    pipeline:
      - runs: |
          mkdir -p "${{targets.contextdir}}/opt/cni/bin"
          ln -sf ../../usr/bin/cni-plugin "${{targets.contextdir}}/opt/cni/bin/cni-plugin"
    test:
      pipeline:
        - uses: test/tw/symlink-check

test:
  pipeline:
    - uses: test/go-fips-check
    - uses: test/tw/ldd-check
```

**CRITICAL: Always include `oldglibc~2.28` for CNI plugins built with go-fips.** This ensures binaries remain ABI compatible when copied to older host operating systems.

## FIPS with MD5 Support

Use `go-fips-md5` when the project uses MD5 for non-cryptographic purposes:

```yaml
pipeline:
  - uses: go/build
    with:
      packages: ./cmd/project
      output: project-name
      go-package: go-fips-md5
      ldflags: -X main.version=${{package.version}}
```

## CGO and Symbol Requirements

FIPS Go binaries MUST be dynamically linked to OpenSSL. Key requirements:

1. **Never set `CGO_ENABLED=0`** for FIPS packages
2. **Never use `-ldflags -s`** (strips symbols needed for FIPS verification)
3. **Never use `-extldflags -static`** (prevents dynamic linking)

### Removing CGO_ENABLED=0 from Upstream

If upstream sets `CGO_ENABLED=0` in Makefile:

```yaml
pipeline:
  - uses: git-checkout
    ...

  - uses: go/remove-cgo-enabled-0
    with:
      files: Makefile

  - uses: go/build
    ...
```

For multiple files or custom sed patterns:

```yaml
- uses: go/remove-cgo-enabled-0
  with:
    files: "Makefile scripts/build.sh"
    seds: |
      s,CGO_ENABLED=0[ ]*,,g
      s,-extldflags.*static,,g
```

## FIPS Runtime Dependencies

For packages using `go-fips` or `go-fips-md5`, the runtime dependency on `openssl-config-fipshardened` is automatically added by melange's static analysis. You do NOT need to add it manually.

## Verifying FIPS Compliance

The `test/go-fips-check` pipeline verifies:
1. Binary uses CGO OpenSSL cryptography symbols
2. Binary is dynamically linked (not static)
3. Binary has `microsoft_systemcrypto=1` (Go 1.25+)

## FIPS Packages and `provides`

**Do NOT add `provides: project-name` to FIPS packages.** FIPS packages are separate packages, not drop-in replacements for non-FIPS versions.

**When `provides` IS used:**
- **Version-streamed packages only**: e.g., `apm-server-fips-8.17` provides `apm-server-fips` (the unversioned FIPS name)
- This allows multiple version streams to satisfy a dependency on the unversioned name

**When `provides` is NOT used:**
- Simple FIPS packages without version streams (e.g., `age-fips`, `addon-resizer-fips`)
- FIPS packages should NOT provide the non-FIPS name

```yaml
# WRONG - do not do this for simple FIPS packages
package:
  name: project-fips
  dependencies:
    provides:
      - project=${{package.full-version}}  # NO!

# CORRECT - only for version-streamed FIPS packages
package:
  name: project-fips-1.2
  dependencies:
    provides:
      - project-fips=${{package.full-version}}  # Provides unversioned FIPS name
```

## Checklist

Before submitting FIPS packages:

- [ ] Uses `go-package: go-fips` or `go-package: go-fips-md5`
- [ ] No `CGO_ENABLED=0` in environment or upstream (use `go/remove-cgo-enabled-0` if needed)
- [ ] No `-ldflags -s` or `-extldflags -static`
- [ ] `test/go-fips-check` in test pipeline
- [ ] NO `provides` for non-FIPS name (only version-streamed packages use `provides`)

**CNI Plugins (FIPS):**
- [ ] Includes `oldglibc~2.28` in build environment
- [ ] Compat subpackage with `/opt/cni/bin` symlink
