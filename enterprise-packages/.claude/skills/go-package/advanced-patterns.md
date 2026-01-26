# Advanced Go Packaging Patterns

This document covers complex Go packaging scenarios including multi-binary packages, workspaces, data lists, and custom build configurations.

## Multi-Binary Package

```yaml
package:
  name: multi-tool
  version: "X.Y.Z"
  epoch: 0
  description: Multi-binary tool suite
  copyright:
    - license: Apache-2.0

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/multi-tool
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  # Build each binary separately
  - uses: go/build
    with:
      packages: ./cmd/tool-one
      output: tool-one
      go-package: go-fips
      ldflags: -X main.version=${{package.version}}

  - uses: go/build
    with:
      packages: ./cmd/tool-two
      output: tool-two
      go-package: go-fips
      ldflags: -X main.version=${{package.version}}

test:
  pipeline:
    - uses: test/go-fips-check
    - uses: test/tw/ldd-check
    - uses: test/tw/ver-check
      with:
        bins: tool-one tool-two
```

## Multi-Component with Data Lists

For projects with many similar components, use data lists:

```yaml
data:
  - name: components
    items:
      component-one: "First component description"
      component-two: "Second component description"

subpackages:
  - range: components
    name: "${{package.name}}-${{range.key}}"
    description: ${{range.value}}
    pipeline:
      - uses: go/build
        with:
          packages: ./cmd/${{range.key}}
          output: ${{range.key}}
          go-package: go-fips
    test:
      pipeline:
        - uses: test/go-fips-check
        - uses: test/tw/ldd-check
```

## Double Range for Binaries + Compat (Dapr Pattern)

Reuse the same data list twice - once for binaries, once for their compat packages:

```yaml
data:
  - name: binaries
    items:
      daprd: "Daprd side car"
      operator: "Operator service"

subpackages:
  # First range: Build each binary
  - range: binaries
    name: "${{package.name}}-${{range.key}}-fips"
    description: ${{range.value}}
    pipeline:
      - uses: go/build
        with:
          go-package: go-fips
          packages: ./cmd/${{range.key}}
          output: ${{range.key}}
    test:
      pipeline:
        - uses: test/go-fips-check

  # Second range: Create compat for each binary (same data list!)
  - range: binaries
    name: "${{package.name}}-${{range.key}}-fips-compat"
    description: Compat for ${{range.key}}
    pipeline:
      - runs: |
          mkdir -p "${{targets.subpkgdir}}"
          ln -sf ./usr/bin/${{range.key}} "${{targets.subpkgdir}}/${{range.key}}"
    test:
      pipeline:
        - uses: test/tw/symlink-check
```

**Key insight:** Using `range: binaries` twice avoids duplicating the component list.

## Version Streams with var-transforms

```yaml
vars:
  major-version: 3

var-transforms:
  - from: ${{package.version}}
    match: ^(\d+\.\d+)\.\d+$
    replace: "$1"
    to: major-minor-version

subpackages:
  - name: ${{package.name}}-${{vars.major-minor-version}}
    description: Versioned subpackage
    dependencies:
      provides:
        - project-name=${{package.full-version}}
```

## Go Workspaces (go.work)

For monorepos using Go workspaces:

```yaml
pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/monorepo
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  # Update go.work to match environment Go version
  - runs: |
      GO_VERSION=$(grep '^go ' go.mod | awk '{print $2}')
      sed -i "s/go [0-9]\+\.[0-9]\+\.[0-9]\+/go ${GO_VERSION}/" go.work

  - uses: go/build
    with:
      packages: ./cmd/project
      output: project
```

For `go/bump` with workspace support, add `work: true`.

## Working Directory for Multi-Binary Builds

Build different binaries from different directories:

```yaml
pipeline:
  - uses: git-checkout
    ...

  # Build main binary from root
  - uses: go/build
    with:
      packages: ./cmd/main
      output: main-binary

  # Build helper from subdirectory
  - working-directory: tools/helper
    uses: go/build
    with:
      packages: .
      output: helper-binary
      tags: helper_tag
```

## Custom vendor.mod Handling (Docker/Moby Pattern)

For projects using non-standard vendoring:

```yaml
pipeline:
  - uses: git-checkout
    ...

  # Convert vendor.mod to go.mod temporarily
  - runs: |
      mv vendor.mod go.mod
      go mod tidy

  - uses: go/bump
    with:
      deps: golang.org/x/crypto@v0.45.0

  # Restore vendor.mod and re-vendor
  - runs: |
      mv go.mod vendor.mod
      mv go.sum vendor.sum
      ./hack/vendor.sh

  - uses: go/build
    with:
      packages: ./cmd/docker
      output: docker
      vendor: true
```

## go/build Options Quick Reference

```yaml
# Custom build tags
- uses: go/build
  with:
    packages: ./cmd/project
    output: project-name
    tags: fips,osusergo,netgo

# Vendor mode (for vendored deps)
- uses: go/build
  with:
    packages: ./cmd/project
    output: project-name
    vendor: true

# Custom modroot (go.mod not in root)
- uses: go/build
  with:
    modroot: cluster-autoscaler
    packages: .
    output: cluster-autoscaler
```

## go generate in Build Pipeline

```yaml
pipeline:
  - uses: git-checkout
    ...

  # Run code generation before build
  - runs: go generate ./...

  - uses: go/build
    with:
      packages: ./cmd/project
      output: project
```

## GODEBUG for Compatibility

```yaml
# Via go.mod
- runs: echo "godebug x509negativeserial=1" >> go.mod

# Or via environment
environment:
  environment:
    GODEBUG: "x509negativeserial=1,http2server=0"
```

**Important:**
- **NEVER** use `CGO_ENABLED=0` for FIPS packages
- **Prefer dynamic linking** even for non-FIPS packages unless explicitly required
- Avoid `-ldflags="-s -w"` which strips symbols

## Kubernetes-Style Makefile Builds

For large projects with their own build systems:

```yaml
vars:
  components: "kubectl kubelet kube-proxy"

pipeline:
  - uses: git-checkout
    ...

  - runs: |
      export FORCE_HOST_GO=true
      export KUBE_GIT_VERSION=v${{package.version}}

      WHAT=""
      for c in ${{vars.components}}; do
        WHAT="$WHAT cmd/$c"
      done
      make WHAT="$WHAT"

      for c in ${{vars.components}}; do
        install -Dm755 _output/bin/$c "${{targets.destdir}}"/usr/bin/$c
      done
```

## Complex ldflags with Nested Substitution

```yaml
- uses: go/build
  with:
    packages: .
    output: rancher
    ldflags: |
      -X main.VERSION=${{package.version}}
      -X github.com/org/project/pkg/version.GitCommit=$(git rev-parse --short HEAD)
      -X github.com/org/project/pkg/settings.InjectDefaults="{\"version\":\"$(grep -m1 'github.com/org/component' go.mod | awk '{print $2}')\"}"
```

## Multiple go.mod Files (Monorepo)

For projects with multiple modules, use `modroot` with each `go/bump`:

```yaml
pipeline:
  - uses: git-checkout
    ...

  - uses: go/bump
    with:
      deps: golang.org/x/crypto@v0.45.0
      modroot: .

  - uses: go/bump
    with:
      deps: golang.org/x/crypto@v0.45.0
      modroot: sdk

  - uses: go/build
    with:
      modroot: .
      packages: ./cmd/server
      output: server

  - uses: go/build
    with:
      modroot: sdk
      packages: ./cmd/cli
      output: cli
```

## Data Lists for Multi-Component Monorepos

For building many binaries from different paths:

```yaml
data:
  - name: components
    items:
      server: "./cmd/server"
      worker: "./cmd/worker"
      cli: "./cmd/cli"

subpackages:
  - range: components
    name: "${{package.name}}-${{range.key}}"
    description: "${{range.key}} component"
    pipeline:
      - uses: go/build
        with:
          packages: ${{range.value}}
          output: ${{range.key}}
          ldflags: -X main.version=${{package.version}}
    test:
      pipeline:
        - uses: test/tw/ldd-check
```

## CGO with Custom Compiler Flags

```yaml
environment:
  contents:
    packages:
      - openssl-dev
      - sqlite-dev
  environment:
    CGO_ENABLED: "1"
    CGO_CFLAGS: "-Os -I/usr/include/"
    CGO_LDFLAGS: "-L/usr/lib/ -lssl -lcrypto"

pipeline:
  - uses: go/build
    with:
      packages: ./cmd/project
      output: project
```
