# Image Mapping

The `images:` and `test:` inputs to `charts/package` declare how container images are configured in a Helm chart and how to validate them.

## The `images:` Block

Maps `<image-key>`s to their helm values paths. The pipeline writes this to `cg.json` in the packaged chart.

```yaml
images: |
  <image-key>:
    values:
      <helm.values.path>:
        <field>: ${marker}
```

### Finding Values Paths

Examine the chart's `values.yaml`:

```yaml
# values.yaml
image:
  registry: docker.io
  repository: nginx
  tag: latest
  digest: ""

metrics:
  enabled: false
  image:
    repository: prom/nginx-exporter
    tag: v0.11.0
```

Maps to:

```yaml
images: |
  nginx:
    values:
      image:
        registry: ${registry}
        repository: ${repo}
        tag: ${tag}
        digest: ${digest}
  nginx-exporter:
    values:
      metrics:
        image:
          repository: ${registry_repo}
          tag: ${pseudo_tag}
```

### Choosing Markers

**Chart has separate registry and repository:**
```yaml
registry: ${registry}
repository: ${repo}
```

**Chart has combined repository (includes registry):**
```yaml
repository: ${registry_repo}
```

**Chart has tag AND digest/sha field:**
```yaml
tag: ${tag}
digest: ${digest}  # or sha: ${digest} - use chart's field name
```

**Chart has NO digest field:**
```yaml
tag: ${pseudo_tag}  # Embeds digest in tag: "latest@sha256:..."
```

**Chart uses single ref field:**
```yaml
image: ${ref}
```

### Common Mistake

Never combine `${pseudo_tag}` with `${digest}`:
```yaml
# WRONG - causes double digest
tag: ${pseudo_tag}
digest: ${digest}
```

### Match Field Names

Use the chart's actual field names, even if non-standard:

```yaml
# Chart uses "sha" instead of "digest"
images: |
  main:
    values:
      image:
        sha: ${digest}  # Chart's field name, our marker
```

## The `test:` Block

Validates that image mappings work correctly.

```yaml
test: |
  values:
    # Merged into ALL test cases
  ignore:
    # Images to skip during validation
  cases:
    - name: <case-name>
      images: [<expected-image-keys>]
      values:
        # Case-specific values
```

### Test Cases

Each case declares which `<image-key>`s should render for that configuration:

```yaml
test: |
  cases:
    - name: default
      images: [nginx]
    - name: with-metrics
      images: [nginx, nginx-exporter]
      values:
        metrics:
          enabled: true
```

The test pipeline:
1. Substitutes markers with test values (e.g., `${registry}` → `cgr.test`)
2. Runs `helm template` with case values
3. Verifies expected images appear (marker patterns present)
4. Fails if unexpected hardcoded images appear

### Global Test Values

Values under `test.values` apply to all cases. Common use:

```yaml
test: |
  values:
    imagePullSecrets:
      - name: pull-secret
```

### Ignore List

Some images can't be validated via marker detection. Use `ignore` with justification:

```yaml
test: |
  ignore:
    - image: "{{ .ProxyImage }}"
      reason: Sidecar injection template - resolved at runtime, not Helm
    - image: docker.io/bitnami/postgresql:16.6.0
      reason: External dependency chart, tested separately
  cases:
    - name: default
      images: [main]
```

**When to use ignore:**

- **Injection templates** - Go templates in ConfigMaps processed at runtime (e.g., Istio's `{{ .ProxyImage }}`)
- **External dependencies** - Images from subchart dependencies tested separately

The ignore list uses exact string matching against extracted image values.

## Discovering Images

### Step 1: Check values.yaml

```bash
grep -n "repository:" values.yaml
grep -n "image:" values.yaml
```

### Step 2: Render and Find All Images

```bash
# Default configuration
helm template . --kube-version 1.35.0 | grep -E "^\s+image:"

# With optional features
helm template . --kube-version 1.35.0 --set metrics.enabled=true | grep -E "^\s+image:"
helm template . --kube-version 1.35.0 --set sidecar.enabled=true | grep -E "^\s+image:"
```

### Step 3: Trace to Values

For each image found, identify which values control it:

```bash
grep -rn "image:" templates/
```
