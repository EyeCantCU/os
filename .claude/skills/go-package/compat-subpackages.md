# Compat Subpackage Patterns

Compat subpackages place binaries where upstream Dockerfiles and Helm charts expect them. **Always check the upstream Dockerfile's ENTRYPOINT/CMD** to determine the correct path.

## How to Identify Required Compat Path

1. **Check upstream Dockerfile**: Look for `ENTRYPOINT` or `CMD`
2. **Check Helm charts**: Look for `command:` or container specs
3. **Check deployment manifests**: Look for binary paths in args

## Common Entrypoint Patterns

| Upstream Path | Use Case | Example Projects |
|---------------|----------|------------------|
| `/manager` | Kubernetes operators | cert-manager, actions-runner-controller |
| `/usr/local/bin/` | Standard Helm charts | ArgoCD, Helm plugins |
| `/opt/cni/bin/` | CNI plugins | Calico, Cilium, Multus |
| `/app` | Generic applications | Many web services |
| `/init` | Init containers | Amazon K8s CNI |
| `/opt/ansible` | Ansible operators | Kiali operator |
| `/home/runner` | GitHub Actions | actions-runner |
| `/fluent-bit/bin/` | Logging agents | NewRelic Fluent Bit |

## Pattern 1: Root-Level Manager (`/manager`)

Most Kubernetes operators use `/manager` as the entrypoint:

```yaml
subpackages:
  - name: ${{package.name}}-compat
    description: Compat package to place binaries in location expected by upstream helm charts
    pipeline:
      - runs: |
          mkdir -p "${{targets.subpkgdir}}"
          # Relative symlink from root: ./usr/bin/manager
          ln -sf ./usr/bin/manager "${{targets.subpkgdir}}/manager"
    test:
      pipeline:
        - uses: test/tw/symlink-check
```

For operators with multiple binaries:

```yaml
subpackages:
  - name: ${{package.name}}-compat
    description: Compat package for upstream helm charts
    pipeline:
      - runs: |
          mkdir -p "${{targets.subpkgdir}}"
          ln -sf ./usr/bin/manager "${{targets.subpkgdir}}/manager"
          ln -sf ./usr/bin/webhook "${{targets.subpkgdir}}/webhook"
          ln -sf ./usr/bin/controller "${{targets.subpkgdir}}/controller"
    test:
      pipeline:
        - uses: test/tw/symlink-check
```

## Pattern 2: Standard Path (`/usr/local/bin/`)

For Helm charts expecting binaries in `/usr/local/bin/`:

```yaml
subpackages:
  - name: ${{package.name}}-compat
    description: Compat package for upstream helm charts
    pipeline:
      - runs: |
          mkdir -p "${{targets.contextdir}}/usr/local/bin"
          # Use relative symlink when possible
          ln -sf ../../bin/project-name "${{targets.contextdir}}/usr/local/bin/project-name"
    test:
      pipeline:
        - uses: test/tw/symlink-check
        - uses: test/tw/ver-check
          with:
            bins: /usr/local/bin/project-name
```

**ArgoCD special case** - must COPY binary (not symlink) because Helm charts copy it between containers:

```yaml
subpackages:
  - name: ${{package.name}}-compat
    description: Compat package for binaries location expected by upstream helm charts
    pipeline:
      - runs: |
          mkdir -p "${{targets.subpkgdir}}/usr/local/bin"
          # MUST be copied (not symlinked) - ArgoCD charts copy between containers
          cp "${{targets.destdir}}/usr/bin/argocd" "${{targets.subpkgdir}}/usr/local/bin/argocd"
          # Relative symlinks within same directory
          ln -s argocd "${{targets.subpkgdir}}/usr/local/bin/argocd-server"
          ln -s argocd "${{targets.subpkgdir}}/usr/local/bin/argocd-repo-server"
```

## Pattern 3: CNI Plugins (`/opt/cni/bin/`)

CNI plugins must be in `/opt/cni/bin/` for Kubernetes to find them:

```yaml
subpackages:
  - name: ${{package.name}}-cni-compat
    description: Compat package for CNI bin directory
    dependencies:
      provides:
        - project-cni-compat=${{package.full-version}}
    pipeline:
      - runs: |
          mkdir -p "${{targets.subpkgdir}}/opt/cni/bin"
          # Relative: from /opt/cni/bin/ up 3 levels to / then down to usr/bin/
          ln -sf ../../../usr/bin/project-cni "${{targets.subpkgdir}}/opt/cni/bin/project-cni"
          ln -sf ../../../usr/bin/project-cni "${{targets.subpkgdir}}/opt/cni/bin/project-ipam"
    test:
      pipeline:
        - uses: test/tw/symlink-check
        - uses: test/virtualpackage
          with:
            virtual-pkg-name: project-cni-compat
            real-pkg-name: ${{subpkg.name}}
```

## Pattern 4: App Root (`/app`)

For services expecting binary at `/app`:

```yaml
subpackages:
  - name: ${{package.name}}-compat
    description: Compat package for upstream Docker image
    pipeline:
      - runs: |
          mkdir -p "${{targets.contextdir}}/app"
          # Relative: from /app/ up 1 level to / then down to usr/bin/
          ln -sf ../usr/bin/project-name "${{targets.contextdir}}/app/project-name"
    test:
      pipeline:
        - uses: test/tw/symlink-check
```

## Pattern 5: Directory Symlink (Full Structure)

For projects expecting entire directory structure (e.g., GitHub Actions runner):

```yaml
subpackages:
  - name: ${{package.name}}-compat
    description: Compat package for upstream Docker image
    pipeline:
      - runs: |
          mkdir -p "${{targets.contextdir}}/home"
          # Relative: from /home/ up 1 level to / then down to usr/share/
          ln -sf ../usr/share/actions-runner "${{targets.contextdir}}/home/runner"
    test:
      pipeline:
        - uses: test/tw/symlink-check
        - runs: |
            # Verify structure inside
            stat /home/runner/externals
            stat /home/runner/run.sh
```

## Pattern 6: Init Container with Hybrid Approach

For init containers needing symlinks AND copies:

```yaml
subpackages:
  - name: ${{package.name}}-init-compat
    description: Compat package for init container
    pipeline:
      - runs: |
          mkdir -p "${{targets.subpkgdir}}/init"
          # Relative: from /init/ up 1 level to / then down to usr/bin/
          ln -sf ../usr/bin/cni-init "${{targets.subpkgdir}}/init/cni-init"
          # CNI plugins must be copied (they get moved during init)
          for bin in bridge host-local loopback portmap; do
            cp /usr/bin/"$bin" "${{targets.subpkgdir}}/init/$bin"
          done
    test:
      pipeline:
        - uses: test/tw/symlink-check
        - runs: |
            stat /init/bridge
            stat /init/host-local
```

## Pattern 7: Vendor-Specific Paths

For Ansible operators (`/opt/ansible`):

```yaml
subpackages:
  - name: ${{package.name}}-compat
    description: Compat package for Ansible operator
    dependencies:
      provides:
        - project-operator-compat=${{package.full-version}}
    pipeline:
      - runs: |
          mkdir -p "${{targets.contextdir}}/opt"
          # Relative: from /opt/ up 1 level to / then down to usr/share/
          ln -sf ../usr/share/project-operator "${{targets.contextdir}}/opt/ansible"
    test:
      pipeline:
        - uses: test/tw/symlink-check
```

## Pattern 8: Virtual Package Compat (Multiple Versions)

When multiple version streams exist:

```yaml
subpackages:
  - name: ${{package.name}}-compat
    description: Compat package for project-name
    dependencies:
      provides:
        - project-name-compat=${{package.full-version}}
    pipeline:
      - runs: |
          mkdir -p "${{targets.contextdir}}/app"
          # Relative: from /app/ up 1 level to / then down to usr/bin/
          ln -sf ../usr/bin/project-name "${{targets.contextdir}}/app/project-name"
    test:
      pipeline:
        - uses: test/tw/symlink-check
        - uses: test/virtualpackage
          with:
            virtual-pkg-name: project-name-compat
            real-pkg-name: ${{subpkg.name}}
```

## Testing Best Practices

1. **Use `test/tw/symlink-check`** - Primary validation for all symlinks in package
2. **Use `readlink -v`** - Shows symlink target in output (helpful for debugging)
3. **Use `stat`** - Better error messages than `test -f` for existence checks
4. **Use `test/virtualpackage`** - For versioned compat packages with provides
5. **Use `test/tw/ver-check` and `test/tw/help-check`** - Test binary functionality via compat path

```yaml
test:
  pipeline:
    - uses: test/tw/symlink-check
    - uses: test/tw/ver-check
      with:
        bins: /manager
    - uses: test/tw/help-check
      with:
        bins: /manager
```

## When to Use Copy vs Symlink

| Scenario | Method | Reason |
|----------|--------|--------|
| Normal compat | Symlink | Saves space, follows updates |
| Multi-container copy (ArgoCD) | Copy | Helm charts copy between containers |
| Init container plugins | Copy | Plugins get moved during init |
| Config files | Copy | May need modifications |
| Everything else | Symlink | Default choice |

## Checklist

- [ ] Checked upstream Dockerfile for ENTRYPOINT/CMD path
- [ ] Checked Helm charts for expected binary locations
- [ ] Correct compat path used (`/manager`, `/usr/local/bin`, `/opt/cni/bin`, `/app`, etc.)
- [ ] Symlink vs copy decision made (copy for ArgoCD-style multi-container scenarios)
- [ ] `test/tw/symlink-check` included
- [ ] Virtual provides added for versioned compat packages
- [ ] `readlink -v` or `stat` used in tests (not `test -f`)
