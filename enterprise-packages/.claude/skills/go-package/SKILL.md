---
name: go-package
description: Create melange packages for Go projects. Use when packaging Go binaries, CLI tools, daemons, Kubernetes operators, CNI plugins, or services built with Go. Covers both standard and FIPS-compliant packages.
---

# Go Package Creation Skill

Follow CLAUDE.md "Package Test Best Practices" and "Packaging Best Practices" sections. This skill covers Go-specific workflow.

## 1. Investigate Before Writing YAML

### Identify Project Type

Check the upstream repository to determine what the project produces:

1. **Check go.mod** for module path and Go version requirements
2. **Check for Docker image** - if upstream publishes one, you'll likely need a `-compat` subpackage
3. **Check if it's a service** - look for config file examples, systemd units, or API documentation
4. **Check for multiple binaries** - look in `cmd/` directory for multiple entrypoints

| Type | What to Look For | Subpackages Needed |
|------|------------------|-------------------|
| **CLI Tool** | Single binary in `cmd/` or root | `-compat` (if Docker image exists) |
| **Daemon/Service** | Config files, `server` subcommand, listens on ports | `-compat`, test with `daemon-check-output` |
| **Kubernetes Operator** | CRDs, controllers, RBAC manifests | `-compat`, test with `test/kwok/cluster` |
| **CNI Plugin** | Network plugin, placed in `/opt/cni/bin` | Requires `oldglibc` for FIPS - see [fips.md](fips.md) |
| **Multi-binary** | Multiple directories in `cmd/` | Split binaries into subpackages - see [advanced-patterns.md](advanced-patterns.md) |
| **UI-Embedded** | `webui/`, `ui/`, or frontend directories; npm/yarn/pnpm | See [embedded-ui.md](embedded-ui.md) |

### Check Build Requirements

1. **go.mod**: Check Go version and dependencies
2. **CGO dependencies**: Look for C library dependencies (sqlite, openssl, etc.)
3. **Build tags**: Check Makefile or CI for required build tags
4. **ldflags**: Look for version injection patterns in Makefile/goreleaser

### Common Build Dependencies

| Dependency Type | Build Packages Needed |
|-----------------|----------------------|
| Basic Go | None - `go/build` handles it automatically |
| Go FIPS | Use `go-package: go-fips` option (don't add `go` or `go-fips` as package) |
| CGO with OpenSSL | Add `openssl-dev` |
| CGO with SQLite | Add `sqlite-dev` |
| Protobuf | Add `protoc`, `protobuf-dev` |
| UI assets | Add `nodejs`, `npm` or `yarn` - see [embedded-ui.md](embedded-ui.md) |

**CRITICAL: Never include `go` as a build dependency when using the `go/build` pipeline. The pipeline handles Go automatically.**

### Check Upstream Release Build

Look for build configuration in:
- **Makefile**: Check for ldflags, build tags, CGO settings
- **Dockerfile**: Build arguments and environment variables
- **goreleaser.yaml**: Build configuration, ldflags, hooks
- **CI files**: `.github/workflows/release*.yml`

### License

- Check `LICENSE`, `COPYING`, or `README` files
- Must be Open-Source license: https://opensource.org/licenses

## 2. Templates

### Standard Go Binary (Non-FIPS)

```yaml
package:
  name: project-name
  version: "X.Y.Z"
  epoch: 0
  description: Short description
  copyright:
    - license: Apache-2.0

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
      ldflags: |
        -X github.com/org/project/pkg/version.Version=${{package.version}}
        -X github.com/org/project/pkg/version.GitCommit=$(git rev-parse HEAD)

subpackages:
  - name: ${{package.name}}-compat
    description: Compat package for upstream Docker image
    pipeline:
      - runs: |
          mkdir -p "${{targets.contextdir}}"
          ln -sf ./usr/bin/project-name "${{targets.contextdir}}/project-name"
    test:
      pipeline:
        - uses: test/tw/symlink-check

update:
  enabled: true
  github:
    identifier: org/project
    strip-prefix: v

test:
  pipeline:
    - uses: test/tw/ldd-check
    - uses: test/tw/ver-check
      with:
        bins: project-name
    - uses: test/tw/help-check
      with:
        bins: project-name
    # REQUIRED: Add functional tests - see Section 3
```

### FIPS Go Package

For FIPS packages, see [fips.md](fips.md) for complete guidance including:
- `go-fips` vs `go-fips-md5` selection
- CNI plugin requirements (`oldglibc`)
- FIPS testing with `test/go-fips-check`
- Rules for `provides` in FIPS packages

Quick reference:

```yaml
package:
  name: project-name-fips
  version: "X.Y.Z"
  epoch: 0
  description: FIPS-enabled project-name
  copyright:
    - license: Apache-2.0

# NOTE: No environment.contents.packages needed for pure go/build pipeline
# NOTE: Do NOT add `provides: project-name` - FIPS packages are separate

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
      go-package: go-fips  # Required for FIPS
      ldflags: -X main.version=${{package.version}}

test:
  pipeline:
    - uses: test/go-fips-check  # Required for FIPS packages
    - uses: test/tw/ldd-check
    # Add functional tests
```

## 3. Functional Test Coverage

**Tests must validate real functionality, not just "--version" or "--help" output.**

### How to Identify What to Test

1. **Read the project description** - what does it claim to do?
2. **Check subcommands** - `tool help` shows available commands
3. **Look at the API** - HTTP endpoints, gRPC services
4. **Check the config** - what features can be configured?

### By Project Type

| Type | Must Test |
|------|-----------|
| **CLI Tool** | Core operations with real input, verify output |
| **Daemon/Service** | Start service, test API endpoints, verify responses |
| **Kubernetes Operator** | Use `test/kwok/cluster`, test controller reconciliation |
| **CNI Plugin** | Test plugin invocation, verify network setup |
| **Proxy/Gateway** | Connection handling, routing, health checks |

### Test Patterns

**Basic CLI Testing:**
```yaml
test:
  pipeline:
    - uses: test/go-fips-check  # Only for FIPS packages
    - uses: test/tw/ldd-check
    - runs: |
        # Verify version
        project-name version
        # Test actual functionality
        echo '{"test": "data"}' | project-name process --format json
        project-name validate --config /etc/project/config.yaml
```

**Daemon Testing with daemon-check-output:**
```yaml
test:
  pipeline:
    - uses: test/go-fips-check  # Only for FIPS packages
    - uses: test/daemon-check-output
      with:
        setup: |
          tee /tmp/config.yaml << 'EOF'
          server:
            port: 8080
            host: 0.0.0.0
          EOF
        start: project-name serve --config /tmp/config.yaml
        timeout: 60
        expected_output: "Server listening on"
        post: |
          set -o pipefail
          curl -sf http://localhost:8080/health | grep -F "healthy"
          curl -sf http://localhost:8080/metrics | grep -F "uptime"
```

**Kubernetes Operator Testing:**
```yaml
test:
  pipeline:
    - uses: test/go-fips-check  # Only for FIPS packages
    - uses: test/kwok/cluster
    - runs: |
        # Start operator in background
        operator --kubeconfig ~/.kube/config --metrics-addr :8081 &
        sleep 10
        # Verify metrics endpoint
        curl -sf http://localhost:8081/metrics | grep -F "controller_runtime"
```

## 4. Version Injection Patterns

### Standard ldflags Pattern

```yaml
ldflags: |
  -X github.com/org/project/pkg/version.Version=${{package.version}}
  -X github.com/org/project/pkg/version.GitCommit=$(git rev-parse HEAD)
  -X github.com/org/project/pkg/version.BuildDate=$(date -d@${SOURCE_DATE_EPOCH} -u +"%Y-%m-%dT%H:%M:%SZ")
```

### Git Describe Pattern

```yaml
ldflags: -X main.version=$(git describe --long --tags --match="v*" --dirty 2>/dev/null || git rev-list -n1 HEAD)
```

**FIPS ldflags Restrictions** - See [fips.md](fips.md):
- Never use `-s` (strips symbols)
- Never use `-w` (strips DWARF)
- Never use `-extldflags -static`

## 5. Update Configuration

### GitHub Releases (Preferred)

```yaml
update:
  enabled: true
  github:
    identifier: org/project
    strip-prefix: v
```

### GitHub Tags (When No Releases)

```yaml
update:
  enabled: true
  github:
    identifier: org/project
    use-tag: true
    strip-prefix: v
```

### Version Stream Filtering

```yaml
update:
  enabled: true
  github:
    identifier: org/project
    strip-prefix: v
    tag-filter-prefix: v1.28.
  ignore-regex-patterns:
    - "-rc"
    - "-beta"
    - "-alpha"
```

## 6. Build and Test

After creating the YAML file, use the `build-test` skill:

1. Build dependencies first (if any)
2. Build the package: `make package/NAME`
3. Run tests: `make test/NAME`
4. Scan for CVEs: `wolfictl scan packages/**/NAME-*.apk`
5. If CVEs found, use `go/bump` - see [cve-remediation.md](cve-remediation.md)

## 7. Quick Reference

| Topic | Reference |
|-------|-----------|
| FIPS packages, CNI plugins, go-fips-md5 | [fips.md](fips.md) |
| CVE remediation with go/bump | [cve-remediation.md](cve-remediation.md) |
| Projects with embedded web UI | [embedded-ui.md](embedded-ui.md) |
| Compat subpackage patterns | [compat-subpackages.md](compat-subpackages.md) |
| Multi-binary, workspaces, data lists | [advanced-patterns.md](advanced-patterns.md) |

## 8. Checklist

Before submitting:

**Build Configuration:**
- [ ] License verified from go.mod and LICENSE file
- [ ] Correct commit hash (not annotated tag)
- [ ] No `go` or `go-fips` in environment packages (use `go-package` option)
- [ ] `go/build` pipeline used (not raw `go build` commands)
- [ ] Minimal build dependencies

**Testing:**
- [ ] `test/tw/ldd-check` on main package
- [ ] Functional tests covering core features (not just --version)
- [ ] Daemon tests with `test/daemon-check-output` for services
- [ ] Kubernetes tests with `test/kwok/cluster` for operators

**FIPS Packages:** See [fips.md](fips.md#checklist) for complete FIPS checklist

**CVE Remediation:** See [cve-remediation.md](cve-remediation.md) if vulnerabilities found

**Compat Subpackages:** See [compat-subpackages.md](compat-subpackages.md) for patterns

**Update Configuration:**
- [ ] Update section with correct identifier
- [ ] `tag-filter-prefix` for versioned packages
- [ ] `use-tag: true` if no GitHub releases
- [ ] `ignore-regex-patterns` to exclude pre-releases
