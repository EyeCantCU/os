---
name: helm-chart
description: Packages upstream Helm charts as APKs with Chainguard image metadata. Use when creating or modifying chart-*.yaml melange configs, mapping helm values to images, or debugging chart test failures.
allowed-tools:
  - Read
  - Grep
  - Glob
  - Edit
  - Execute
---

# Packaging Upstream Helm Charts

Creates melange packages that wrap upstream Helm charts with Chainguard image metadata (`cg.json`), enabling automated image substitution and validation.

> **Scope:** Non-iamguarded charts only (e.g., `chart-prometheus-kube-state-metrics`). For iamguarded charts like `chart-iamguarded-redis`, use the `iamguarded-chart` skill instead.

## When to Use

- Creating a new `chart-*.yaml` melange config for an upstream Helm chart
- Modifying an existing `chart-*.yaml` (adding images, fixing mappings, updating test cases)
- Mapping container images in a chart to their helm values paths
- Adding or updating test cases to validate image configurability
- Debugging "image not found" or "unconfigurable image" test failures
- Charts that need preprocessing before packaging (source charts)

## When NOT to Use

- **iamguarded charts** - These use a different workflow with the `chainguard-dev/iamguarded-charts` fork
- **Adding chart.cue in images-private** - That's the next step after the APK exists; see the `chart-module` skill in images-private
- **General melange package authoring** - This skill is chart-specific

## Quick Start

```yaml
package:
  name: chart-<source>-<chart-name>
  version: "X.Y.Z"
  epoch: 0
  description: <description from Chart.yaml>
  copyright:
    - license: Apache-2.0

pipeline:
  - uses: git-checkout
    with:
      repository: https://github.com/ORG/REPO
      tag: <tag-pattern>-${{package.version}}
      expected-commit: <COMMIT>

  - uses: charts/package
    with:
      chart-path: charts/<chart-name>
      images: |
        <image-key>:
          values:
            image:
              registry: ${registry}
              repository: ${repo}
              tag: ${tag}
              digest: ${digest}
      test: |
        cases:
          - name: default
            images: [<image-key>]

test:
  pipeline:
    - uses: test/chart-standards

update:
  enabled: true
  github:
    identifier: ORG/REPO
    strip-prefix: <tag-pattern>-
    use-tag: true
    tag-filter-prefix: <tag-pattern>-
```

## Core Concepts

### Why `<image-key>` Matters

The key you choose in `images:` (e.g., `kube-state-metrics`) is a human-readable identifier that flows through the entire system:

1. Declared in `images:` block in melange config
2. Referenced in `test.cases[].images` lists
3. Used in `chart.cue` in images-private to bind to actual Chainguard images

Choose descriptive names. They appear in error messages and must be consistent across all three locations.

### Why Multiple Test Cases

Charts have optional features (metrics exporters, sidecars, RBAC proxies) disabled by default. Different helm values produce different manifests with different images. Each test case:

1. Enables a specific configuration via `values:`
2. Declares which `<image-key>`s should appear in rendered output via `images:`

The test pipeline substitutes markers with test values, runs `helm template`, and verifies:
- Expected images appear (markers present)
- No unexpected hardcoded images appear

Without explicit `images:` declarations, the test can't know which images are expected for each configuration.

### Markers Reference

| Marker | What It Is | Use When |
|--------|------------|----------|
| `${registry}` | Registry hostname (e.g., `cgr.dev`) | Chart has separate registry field |
| `${repo}` | Repository path (e.g., `chainguard/nginx`) | Chart has separate repository field |
| `${registry_repo}` | Full image path (e.g., `cgr.dev/chainguard/nginx`) | Chart has single combined repository field |
| `${tag}` | Image tag (e.g., `latest`) | Chart has a tag field |
| `${digest}` | Content digest (e.g., `sha256:abc...`) | Chart has a digest/sha field |
| `${pseudo_tag}` | Digest disguised as a tag (e.g., `unused@sha256:abc...`) | Chart has NO digest field - this lets you inject a digest via the tag field |
| `${ref}` | Full reference (e.g., `cgr.dev/chainguard/nginx@sha256:abc...`) | Chart uses single field for entire image ref |

**Why `${pseudo_tag}`?** Some charts only have a `tag` field with no `digest` support. Since we need digest pinning for reproducibility, `${pseudo_tag}` is a workaround: it looks like a tag to the chart but actually contains the digest. The "tag" portion is ignored; only the digest matters.

**Critical:** Never combine `${pseudo_tag}` with `${digest}` - causes double digest.

## Workflow

### 1. Investigate the Chart

```bash
# Find tag format
git ls-remote --tags https://github.com/ORG/REPO.git | grep CHART | tail -5

# Clone and explore
cd /tmp && rm -rf chart-src
git clone --depth=1 --branch=<tag> <repo-url> chart-src

# Check for dependencies (must package these first)
cat chart-src/charts/<name>/Chart.yaml | yq '.dependencies'

# Find images in values.yaml
grep -n "repository:" chart-src/charts/<name>/values.yaml

# Render and find all images
helm template chart-src/charts/<name> --kube-version 1.35.0 | grep -E "^\s+image:"

# Find optional images
helm template chart-src/charts/<name> --kube-version 1.35.0 \
  --set metrics.enabled=true | grep -E "^\s+image:"
```

### 2. Check for Source Chart Issues

Some repos don't contain the final chart - they have build steps. Signs:
- `Chart.template.yaml` instead of `Chart.yaml`
- Placeholder versions like `0.0.0`
- Makefile targets for chart generation

If found, see [references/source-charts.md](references/source-charts.md).

### 3. Map Images to Values

Examine `values.yaml` to find image configuration patterns. The structure of your `images:` block should mirror the chart's values structure:

```yaml
# If values.yaml has:        # Your images: block should have:
# image:                     # main-app:
#   registry: docker.io      #   values:
#   repository: nginx        #     image:
#   tag: latest              #       registry: ${registry}
#   sha: ""                  #       repository: ${repo}
#                            #       tag: ${tag}
#                            #       sha: ${digest}
```

Use the chart's actual field names (e.g., `sha` not `digest` if that's what the chart uses).

See [references/image-mapping.md](references/image-mapping.md) for detailed patterns and examples.

### 4. Build and Test

```bash
make package/chart-<source>-<chart-name>
make test/chart-<source>-<chart-name>
```

## Troubleshooting

### "image 'X' not found in rendered output"

The test case lists an image in `images:` but its markers didn't appear.

**Check:** Are the `values:` in the test case correct to enable that image? Is the values path in `images:` mapping correct?

### "unconfigurable image: docker.io/..."

A hardcoded image appeared that isn't declared in `images:`.

**Options:**
1. Add it to `images:` mapping if it should be Chainguard-replaceable
2. Add to `test.ignore` with reason if it's external/expected
3. Patch the chart if it's truly hardcoded (rare)

See [references/image-mapping.md](references/image-mapping.md) for the `ignore` feature.

## References

- [references/image-mapping.md](references/image-mapping.md) - Detailed image/test configuration
- [references/source-charts.md](references/source-charts.md) - Charts requiring preprocessing
- [references/patches.md](references/patches.md) - When and how to patch (rare)

## Golden Example

[chart-prometheus-kube-state-metrics.yaml](https://github.com/chainguard-dev/enterprise-packages/blob/main/chart-prometheus-kube-state-metrics.yaml) - Simple chart with main image + optional sidecar, two test cases.

## Next Step

After the chart APK merges, add `chart.cue` in images-private. See the `chart-module` skill there.
