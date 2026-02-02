# Go Packages with Embedded UI

Many Go projects embed web UI assets into the binary. This requires building frontend assets before Go compilation.

## Investigation Checklist

Before packaging, check:
1. **Frontend framework**: React, Vue, Angular, Ember.js?
2. **Package manager**: npm, yarn, yarn-berry, pnpm?
3. **Build command**: `npm run build`, `make static-dist`, `yarn build`?
4. **Build tags**: Does Go need `ui`, `builtinassets`, `webassets_embed` tags?
5. **Asset location**: Where do compiled assets go?

## Common UI Build Patterns

| Project Type | Package Manager | Build Command | Go Build Tags |
|--------------|-----------------|---------------|---------------|
| Vault | npm/yarn | `make static-dist` | `vault,ui` |
| Traefik | yarn-berry | `yarn build` (webui/) | none |
| Prometheus | npm | `make assets-compress` | `netgo,builtinassets` |
| ArgoCD | yarn | `yarn build` (ui/) | none (via Makefile) |
| Grafana | yarn | `make build` | (embedded in build) |
| Jaeger | npm | `npm run build` (jaeger-ui/) | none |
| Teleport | pnpm/wasm-pack | `make ensure-webassets` | `webassets_embed` |

## Template: Simple UI Embedding (Traefik Pattern)

```yaml
environment:
  contents:
    packages:
      - nodejs-20
      - yarn-berry

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  # Build UI FIRST, before Go compilation
  - working-directory: webui
    runs: |
      yarn install
      yarn build

  # Then build Go binary (UI assets are embedded)
  - uses: go/build
    with:
      packages: ./cmd/project
      output: project-name
      go-package: go-fips
      ldflags: -X main.version=${{package.version}}
```

## Template: Complex UI with Build Tags (Vault Pattern)

For projects requiring explicit build tags and Makefile integration:

```yaml
package:
  name: project-fips
  version: "X.Y.Z"
  epoch: 0
  description: Project with embedded UI
  copyright:
    - license: Apache-2.0
  resources:
    cpu: 16
    memory: 48Gi  # UI builds are memory-intensive

environment:
  contents:
    packages:
      # Pin Node.js version - newer versions often break bundlers
      - nodejs-22
      - nofips-node  # Disable FIPS for Node during build
      - npm=10.8.3   # Pin npm for lockfile stability
      - yarn
  environment:
    CGO_ENABLED: "1"
    # Prevent OOM during Ember/webpack builds on aarch64
    NODE_OPTIONS: "--max-old-space-size=8192"
    # Reduce yarn memory usage
    YARN_CACHE_FOLDER: "/tmp/.yarn-cache"
    YARN_ENABLE_GLOBAL_CACHE: "false"

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  - uses: go/bump
    with:
      deps: golang.org/x/crypto@v0.45.0

  # Ensure CGO is enabled for FIPS (upstream may hardcode CGO_ENABLED=0)
  - runs: |
      sed -i 's/CGO_ENABLED=0/CGO_ENABLED=1/' Makefile

  # Build UI assets first
  - runs: |
      # For x86_64, limit parallelism to avoid OOM
      if [[ "${{build.arch}}" == "x86_64" ]]; then
        export JOBS=1
      fi
      # Build frontend assets
      CI=1 make static-dist

  # Build Go with UI tag to include embedded assets
  - uses: go/build
    with:
      packages: .
      output: project-name
      vendor: true
      tags: "project,ui"
      go-package: go-fips-md5
      ldflags: |
        -X github.com/org/project/version.Version=${{package.version}}
        -X github.com/org/project/version.GitCommit=$(git rev-parse HEAD)
```

## Template: Compressed Assets (Prometheus Pattern)

For projects that compress UI assets before embedding (uses direct `go build` instead of pipeline):

```yaml
environment:
  contents:
    packages:
      - bash  # Required for Makefile
      - go-fips-1.25  # Needed since using direct go build
      - nodejs
      - nofips-node
      - npm=10.8.3

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  - uses: go/bump
    with:
      deps: golang.org/x/crypto@v0.45.0

  # Build and compress UI assets, then compile Go
  - runs: |
      GOLDFLAGS="-X github.com/org/project/version.Version=${{package.version}}"

      # Use -j1 to ensure correct Makefile order
      # Actual Go compilation is still parallel
      make -j1 assets-compress

      # Build with builtinassets tag to embed compressed UI
      go build \
        -trimpath \
        -mod=readonly \
        -ldflags "$GOLDFLAGS" \
        -tags netgo,builtinassets \
        ./cmd/project

  - runs: |
      install -Dm755 project "${{targets.destdir}}"/usr/bin/project
```

## Template: Multi-Directory UI Build (ArgoCD Pattern)

For projects with UI in a subdirectory (uses Makefile with direct build):

```yaml
environment:
  contents:
    packages:
      - go-fips  # Needed since using Makefile build
      - nodejs-18
      - nofips-node
      - yarn

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/org/project
      tag: v${{package.version}}
      expected-commit: COMMIT_HASH

  - uses: go/bump
    with:
      deps: golang.org/x/crypto@v0.45.0

  # Build UI in subdirectory
  - runs: |
      cd ui
      yarn install
      yarn cache clean
      NODE_ENV='production' NODE_OPTIONS=--max_old_space_size=8192 yarn build

      cd ..
      # Clear conflicting flags
      unset LDFLAGS GOFLAGS

      # Build Go binary (Makefile handles UI embedding)
      make project-all CGO_FLAG=1 STATIC_BUILD=false

      mkdir -p ${{targets.destdir}}/usr/bin
      mv dist/project* ${{targets.destdir}}/usr/bin/
```

## Template: Conditional UI Build (Jaeger Pattern)

For multi-component projects where only some components need UI:

```yaml
data:
  - name: components
    items:
      all-in-one: "All-in-one distribution with UI"
      collector: "Data collector (no UI)"
      query: "Query service (no UI)"

subpackages:
  - range: components
    name: "${{package.name}}-${{range.key}}"
    description: ${{range.value}}
    pipeline:
      # Only build UI for all-in-one component
      - runs: |
          if [[ "${{range.key}}" = "all-in-one" ]]; then
            mkdir -p jaeger-ui/packages/jaeger-ui/build
            npm install --prefix jaeger-ui/
            cd jaeger-ui/packages/jaeger-ui
            npm run build
            cd ../../..
            # Copy and compress UI assets
            cp -r jaeger-ui/packages/jaeger-ui/build/* cmd/query/app/ui/actual
            find cmd/query/app/ui/actual -type f | xargs gzip --no-name
          fi

      - uses: go/build
        with:
          packages: ./cmd/${{range.key}}
          output: ${{range.key}}
          go-package: go-fips
          ldflags: -X main.version=${{package.version}}
```

## Troubleshooting

### Problem: OOM during webpack/bundler build

```yaml
environment:
  environment:
    NODE_OPTIONS: "--max-old-space-size=8192"
```

### Problem: Node.js FIPS errors during build

```yaml
environment:
  contents:
    packages:
      - nofips-node  # Disables FIPS for Node.js during build
```

### Problem: npm lockfile errors

```yaml
environment:
  contents:
    packages:
      - npm=10.8.3  # Pin npm version for lockfile stability
```

### Problem: Upstream hardcodes CGO_ENABLED=0

```yaml
- runs: |
    sed -i 's/CGO_ENABLED=0/CGO_ENABLED=1/' Makefile
```

### Problem: Build fails on aarch64 but works on x86_64

- Increase resources: `resources: { cpu: 16, memory: 48Gi }`
- Limit parallelism: `JOBS=1`
- Check architecture-specific code paths

## Key Principles

1. **Build order matters**: UI assets MUST be built BEFORE Go compilation
2. **Pin Node.js version**: Newer versions often break bundlers (webpack, Ember)
3. **Use `nofips-node`**: FIPS mode breaks Node.js crypto during builds
4. **Set `NODE_OPTIONS`**: Prevent OOM with `--max-old-space-size=8192`
5. **Check build tags**: Many projects require explicit tags like `ui`, `builtinassets`
6. **CGO for FIPS**: Ensure `CGO_ENABLED=1` is set (may need to patch Makefile)

## Checklist

For packages with embedded UI:

- [ ] Node.js version pinned (avoid latest - bundlers break frequently)
- [ ] `nofips-node` included to disable FIPS during Node build
- [ ] `NODE_OPTIONS: "--max-old-space-size=8192"` set to prevent OOM
- [ ] UI built BEFORE Go compilation in pipeline
- [ ] Correct build tags used (`ui`, `builtinassets`, `webassets_embed`, etc.)
- [ ] `CGO_ENABLED=1` ensured (patch Makefile if upstream sets `CGO_ENABLED=0`)
- [ ] Resources increased if needed: `resources: { cpu: 16, memory: 48Gi }`
