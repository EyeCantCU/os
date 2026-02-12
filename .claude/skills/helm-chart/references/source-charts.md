# Source Charts

Our melange pipeline always starts with `git-checkout` - we build from source, not from published artifacts. This works seamlessly for most charts where the repository contains the final chart directly.

However, some chart repositories don't contain the final chart - they contain source that gets transformed during release via scripts, Makefiles, or other build tooling. When this is the case, we must replicate upstream's build process.

## Why This Matters

If you package the raw source without preprocessing, you'll get:
- Wrong versions in Chart.yaml
- Dev/test defaults instead of production values
- Missing generated files

The packaged chart won't match what users expect from `helm pull`.

## Signs of a Source Chart

- `Chart.template.yaml` instead of `Chart.yaml`
- Placeholder versions like `0.0.0` or `v0.1.0`
- Build scripts or Makefile targets for chart generation
- Dev defaults in values.yaml (e.g., `hub: gcr.io/istio-testing`)

## Approach

**Replicate upstream's build process as closely as possible.** Don't invent your own transformation - study how upstream actually builds and releases their chart, then mirror that in a `runs:` step.

1. Find upstream's release process (Makefile, CI workflows, release scripts)
2. Identify the exact commands that transform source → published chart
3. Replicate those commands in your `runs:` step before `charts/package`
4. Verify your output matches the officially published chart

The goal is for our packaged chart to be identical to what `helm pull` would give users (minus our `cg.json` metadata and vendored dependencies).

### Verifying Against Published Charts

Always compare your preprocessed chart against what upstream publishes:

```bash
# Download published chart
helm pull <chart-name> --repo <repo-url> --version <version> --untar

# Build our package
make package/chart-<name>

# Extract and compare
tar -xf packages/aarch64/chart-*.apk -C /tmp/ours
diff -r <published>/ /tmp/ours/<chart-name>/
```

**Expected differences (acceptable):**
- `cg.json` (our metadata - only in our chart)
- Dependency URLs changed to `file:///` (vendored)

**Unexpected differences** indicate your preprocessing doesn't match upstream.

## Examples

### cert-manager

**Problem:** Source has `Chart.template.yaml` with placeholder version. Makefile generates final chart.

**Solution:** Use upstream's make target:

```yaml
pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/cert-manager/cert-manager
      tag: v${{package.version}}
      expected-commit: <COMMIT>

  - runs: |
      make helm-chart VERSION=v${{package.version}} HELM=/usr/bin/helm YQ=/usr/bin/yq

  - uses: charts/package
    with:
      chart-path: _bin/helm/cert-manager
      chart-version: v${{package.version}}
      images: |
        # ...
```

### Istio

**Problem:** 
- Placeholder versions (`1.0.0`) and dev defaults (`hub: gcr.io/istio-testing`)
- Multiple charts in single repo with different configurations

**Solution:** Use `yq` to patch versions and defaults:

```yaml
pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/istio/istio
      tag: ${{package.version}}
      expected-commit: <COMMIT>

  - runs: |
      for dir in manifests/charts/*/; do
        yq -i '.version = "${{package.version}}"' "$dir/Chart.yaml"
        yq -i '.appVersion = "${{package.version}}"' "$dir/Chart.yaml"
      done
      yq -i '._internal_defaults_do_not_set.global.hub = "cgr.dev/chainguard"' \
        manifests/charts/istiod/values.yaml

  - uses: charts/package
    with:
      chart-path: manifests/charts/istiod
      # ...
```

### Kyverno

**Problem:** Chart source uses placeholder, Makefile generates versioned chart.

**Solution:**

```yaml
pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/kyverno/kyverno
      tag: v${{package.version}}
      expected-commit: <COMMIT>

  - runs: |
      make codegen-helm-docs
      yq -i '.version = "${{package.version}}"' charts/kyverno/Chart.yaml
      yq -i '.appVersion = "v${{package.version}}"' charts/kyverno/Chart.yaml

  - uses: charts/package
    with:
      chart-path: charts/kyverno
      # ...
```

## The `chart-version` Input

If upstream's chart version has a prefix (commonly `v`), use `chart-version`:

```yaml
- uses: charts/package
  with:
    chart-path: charts/myapp
    chart-version: v${{package.version}}  # Produces Chart.yaml version: v1.2.3
```

Without this, `chart-version` defaults to `package.version`.

## Tools Commonly Needed

Add to `environment.contents.packages` if preprocessing needs them:

- `yq` - YAML manipulation
- `make` - Running upstream Makefiles
- `sed` - Simple text substitution
- `helm` - Already included by `charts/package`
