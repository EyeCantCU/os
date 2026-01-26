# Chart Patches

Patches are a last resort. Most charts work without them.

## When Patches Are Actually Needed

1. **Hardcoded images in templates** - Literal image refs instead of values
2. **Missing image fields** - Chart doesn't expose registry/digest as configurable
3. **Broken image assembly** - Templates incorrectly build image references

**NOT a reason to patch:** Chart uses `sha` instead of `digest`. Just use the chart's field name in your `images:` mapping.

## Workflow

### 1. Clone at Exact Tag

```bash
cd /tmp && rm -rf chart-src
git clone --depth=1 --branch=<tag> <repo-url> chart-src
```

### 2. Make Minimal Edits

**values.yaml - Add missing fields:**
```yaml
image:
  registry: ""        # Add
  repository: nginx
  tag: "1.25"
  digest: ""          # Add
```

**templates/_helpers.tpl - Add digest support:**
```yaml
{{- define "chart.image" -}}
{{ .Values.image.repository }}:{{ .Values.image.tag }}
{{- if .Values.image.digest }}@{{ .Values.image.digest }}{{- end }}
{{- end -}}
```

### 3. Generate Patch

```bash
cd /tmp/chart-src
git diff > /path/to/enterprise-packages/chart-<source>-<name>/cg-standards.patch
```

### 4. Add Patch Step

```yaml
pipeline:
  - uses: git-checkout
    with:
      repository: ...
      tag: ...
      expected-commit: ...

  - uses: patch
    with:
      patches: cg-standards.patch

  - uses: charts/package
    with:
      chart-path: ...
```

## Directory Structure

Patches live in a directory matching the package name:

```
enterprise-packages/
  chart-<source>-<chart-name>.yaml
  chart-<source>-<chart-name>/
    cg-standards.patch
```

For example, `chart-grafana-tempo.yaml` would have patches in `chart-grafana-tempo/`.

## Example Patch

Adding digest support to a chart that only has tag:

```diff
diff --git a/charts/myapp/values.yaml b/charts/myapp/values.yaml
--- a/charts/myapp/values.yaml
+++ b/charts/myapp/values.yaml
@@ -10,6 +10,7 @@ image:
   repository: myorg/myapp
   tag: "1.0.0"
+  digest: ""

diff --git a/charts/myapp/templates/_helpers.tpl b/charts/myapp/templates/_helpers.tpl
--- a/charts/myapp/templates/_helpers.tpl
+++ b/charts/myapp/templates/_helpers.tpl
@@ -50,5 +50,6 @@ Create image reference
 {{- define "myapp.image" -}}
-{{ .Values.image.repository }}:{{ .Values.image.tag }}
+{{ .Values.image.repository }}:{{ .Values.image.tag }}{{ if .Values.image.digest }}@{{ .Values.image.digest }}{{ end }}
 {{- end -}}
```

## Troubleshooting

### "Hunk FAILED"

Patch context doesn't match file content.

- Wrong git tag/commit
- Upstream changed since patch was created

**Fix:** Re-clone at correct tag, regenerate patch.

### Patch Applied But Image Still Hardcoded

Template uses the field differently than expected.

**Debug:**
```bash
cd /tmp/chart-src
git apply /path/to/patch
helm template . | grep "image:"
```
