# Claude Code Knowledge Base

This document contains learnings and best practices discovered while working with this repository.

## Packaging Best Practices

### 1. Keep dependencies minimal
**Build dependencies** should be the minimal set required to build the package. **Runtime dependencies** should only include what is necessary for the package to run correctly. Dev dependencies typically should not be used at runtime by packages.

```yaml
# Good - minimal dependencies
environment:
  contents:
    packages:
      - build-base     # Only if compiling C/C++
      - python3        # Only if building Python packages

dependencies:
  runtime:
    - bash             # Only if scripts require bash specifically
```

**Library dependencies:** Libraries like `libssl3` rarely need to be added explicitly at runtime because melange uses static code analysis to determine what libraries are necessary and automatically generates dependencies (e.g., `so:libssl.so.3`).

**Go build dependencies:**
- **NEVER** include `go` as a build dependency when using the `go/build` pipeline
- For FIPS Go packages using `go/build`, pass `go-package: go-fips` as an option:
```yaml
- uses: go/build
  with:
    go-package: go-fips  # Required for FIPS packages
```
- Only include `go` or `go-fips` as build dependencies when NOT using the `go/build` pipeline
- Never set `CGO_ENABLED: 0` in the package's environment for packages built with `go-fips`
- Always include `oldglibc` at buildtime for CNI packages that use `go-fips` to maintain compatibility with hosts using older glibc

**FIPS dependencies**
- All FIPS packages must include `openssl-config-fipshardened` at runtime
  - Exceptions to this are packages built with `go-fips` and packages that use other FIPS validated crypto providers such as `boringssl`.
- All Java FIPS packages must include `openjdk-bcfips-policy-140-3-j${{vars.java-version}}` at runtime
- All FIPS packages must produce binaries and libraries that link against our libssl and libcrypto
- FIPS packages should **never** vendor libssl or libcrypto

**Avoid:**
- Adding build tools to runtime dependencies unless required
- Including "nice to have" packages that aren't required
- Development packages at runtime unless required
- Explicit library dependencies that static code analysis in melange handles automatically

### 2. Use variables and data lists appropriately
If a value is reusable, use a variable. Use clear, descriptive variable names in `vars` section. Use `data` lists only when creating multiple similar subpackages with different names:

```yaml
# Good - variables for reusable values
vars:
  java-version: 17
  package-home: /usr/share/${{package.name}}

# Good - data lists for multiple subpackages with different names
data:
  - name: py-versions
    items:
      3.11: '311'
      3.12: '312'
      3.13: '313'

subpackages:
  - range: py-versions
    name: py${{range.key}}-${{package.name}}  # Creates py3.11-package, py3.12-package, etc.
```

**NEVER** use lists if the subpackage name is the same:

```
# Bad - subpackage name is the same. This will recreate the same subpackage multiple times.
subpackages:
  - range: py-versions
    name: py3-${{package.name}}  # Creates py3-package
```

### 3. Use no-provides and no-depends appropriately
Use `no-provides` and `no-depends` options to manage dependency resolution for packages with vendored libraries:

```yaml
# For Python virtual environments and Ruby bundles
options:
  no-depends: true   # Don't depend on external libraries included in bundles
  no-provides: true  # Don't resolve bundled libraries as providers
```

**Common use cases:**

**Both options together:**
- **Python virtual environments** (e.g., `airflow-2`, `barman`, `superset-4.1`)
- **Ruby bundles** (e.g., `ruby3.3-fluentd-kubernetes-daemonset`)

**`no-depends: true` only:**
- **CUDA/hardware-specific packages** (version-locked dependencies)
- **Database-specific drivers** (prevent version conflicts)

**`no-provides: true` only:**
- **Vendor-specific builds** (e.g., `nginx-bitnami` - avoid conflicts with standard nginx)
- **Legacy/compatibility libraries** (e.g., `oldglibc` - don't provide older libraries with same soname as current ones)

**Always include explanatory comments** when using these options.

### 4. Use virtual provides appropriately
Virtual provides should be added when packages that provide different versions of the same thing conflict, but should **not** be used when multiple streams of a package are co-installable.

```yaml
# Good - conflicting packages that install to the same location
provides:
  - kafka=${{package.full-version}}  # Virtual provide to prevent kafka-3.8 and kafka-3.9 from conflicting
```

**When to use virtual provides:**
- **Conflicting installations**: Packages that install to the same filesystem location (e.g., `kafka-3.8` and `kafka-3.9` both install to `/usr/share/kafka`)
- **Binary name conflicts**: Different version streams of the same package provide the same binary names
- **The latest version of a co-installable stream**: Adding virtual provides to the latest version of a package stream can be helpful for end users that want to be able to easily install the latest version of a package they use. Likewise, this also makes sense for compilers like clang so that packages can be easily rebuilt with the latest version of that compiler
  - This should only be done for the latest version stream as retaining the virtual provides for an older stream will cause the older stream to conflict with the newer stream when installed.

**When NOT to use virtual provides:**
- **Co-installable package streams**: Packages designed to be installed side-by-side (e.g., `postgresql-14` and `postgresql-15`)
- **Python packages**: Different Python versions can coexist (e.g., `py3.12-setuptools` and `py3.13-setuptools`)
  - By providing `py3-setuptools` in `py3.12-setuptools` and `py3.13-setuptools`, it is not impossible to install `py3-setuptools` and `py3.12-setuptools` at the same time

**Examples:**

**Should have virtual provides:**
- `kafka-3.8` and `kafka-3.9` (both vendor Kafka at `/usr/share/kafka`)
- `zookeeper-3.8` and `zookeeper-3.9` (both vendor Zookeeper at `/usr/share/zookeeper`)
- `clang-20` (assuming `clang-20` is the latest version of clang)

**Should NOT have virtual provides:**
- `postgresql-14` and `postgresql-15` (different service ports, can coexist)
- `py3.12-setuptools` and `py3.13-setuptools` (different Python environments)
- `openjdk-21` and `openjdk-17` (installed to different locations, can coexist)
- `clang-19` (as it is not the latest version of Clang)

## Package Test Best Practices

### CRITICAL: Never ignore errors
- **NEVER** use `|| true` to suppress failures
- **NEVER** redirect errors to `/dev/null` without handling them
- **NEVER** use constructs that mask failures
- Tests should fail loudly and clearly when something goes wrong

**Bad patterns to avoid:**
```bash
# DON'T DO THIS - hides failures
some_command || true

# DON'T DO THIS - ignores errors
some_command 2>/dev/null || echo "continuing anyway"

# DON'T DO THIS - continues despite failure
if ! some_command; then
  echo "Oh well, moving on..."
fi
```

### 1. Use `set -euo pipefail` at the start of test scripts
```bash
set -euo pipefail
```
- `-e`: Exit immediately if any command fails
- `-u`: Treat unset variables as an error
- `-o pipefail`: Fail if any command in a pipeline fails (not just the last one)

This ensures tests fail fast and don't mask errors.

### 2. Use `jq` for JSON parsing instead of `grep`/`tail`/`head`
**Bad pattern:**
```bash
RESPONSE=$(curl -k -s -w "\n%{http_code}" "https://api.example.com/endpoint")
HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -n -1)
if [ "$HTTP_CODE" != "200" ]; then
  echo "Failed with HTTP $HTTP_CODE"; exit 1
fi
if ! echo "$BODY" | grep -q '"expected_field"'; then
  echo "Missing expected field"; exit 1
fi
```

**Good pattern:**
```bash
# Use curl -f to fail on HTTP errors
RESPONSE=$(curl -k -s -f "https://api.example.com/endpoint")
# Use jq -e to validate and exit on false conditions
echo "$RESPONSE" | jq -e '.expected_field == "expected_value"'
```

### 3. Use `curl -f` flag for API tests
The `-f` flag makes curl return non-zero exit code on HTTP errors (4xx, 5xx), eliminating the need to manually check status codes. Combined with `set -e`, this provides automatic failure handling.

### 4. Simplify validation with `jq -e`
The `-e` flag makes jq exit with status 1 if the expression evaluates to false or null:
```bash
# Single field validation
echo "$RESPONSE" | jq -e '.userName == "testuser123"'

# Multiple fields validation
echo "$RESPONSE" | jq -e '.issuer and .token_endpoint and .jwks_uri'

# Array contains check
echo "$RESPONSE" | jq -e '.emails | contains(["test@example.com"])'

# Length validation
echo "$RESPONSE" | jq -e '.keys | length > 0'
```

### 5. Use test pipelines
Use specialized test pipelines for different package types to ensure proper validation:

**`test/docs`** - For documentation subpackages:
```yaml
subpackages:
  - name: ${{package.name}}-doc
    test:
      pipeline:
        - uses: test/docs
```
Validates that doc packages contain readable documentation (man pages, info pages, or text files) and aren't empty.

**`test/pkgconf`** - For development subpackages:
```yaml
subpackages:
  - name: ${{package.name}}-dev
    test:
      pipeline:
        - uses: test/pkgconf
```
Validates that dev packages contain properly formatted `.pc` files and that pkgconf can read them.

**`test/daemon-check-output`** - For daemon/service packages:
```yaml
test:
  pipeline:
    - uses: test/daemon-check-output
      with:
        start: "service-name --config /etc/config"
        expected_output: |
          Server started
          Listening on port 8080
```
Starts a daemon, waits for expected output, and checks for error strings.

**`test/metapackage`** - For meta packages:
```yaml
test:
  pipeline:
    - uses: test/metapackage
```
Validates that meta packages are empty, have runtime dependencies, and contain "meta" in their description.

**`test/virtualpackage`** - For virtual packages:
```yaml
test:
  pipeline:
    - uses: test/virtualpackage
      with:
        virtual-pkg-name: ${{package.name}}
        real-pkg-name: actual-implementation
```
Validates that virtual packages aren't installed and that real packages provide them correctly.

**`test/tw/ldd-check`** - For binary packages:
```yaml
test:
  pipeline:
    - uses: test/tw/ldd-check
```
Checks that all binaries have their library dependencies satisfied and no missing shared objects.

### 6. Functional testing patterns
Tests should validate actual functionality, not just version/help output. For examples of comprehensive functional tests, see packages like:
- `postgresql-15` - database creation, read/write operations, service startup
- `pgadmin4` - web interface functionality, user management
- `zitadel` - OAuth2/OIDC workflows, database integration
- `wso2is` - SCIM API testing, SAML validation, user creation
- `valkey-8.0` - key-value operations, Redis compatibility
- `envoy-1.33` - admin endpoints, hot restart, proxy functionality
- `nginx-bitnami` - HTTP server testing, configuration validation
- `prometheus-3.4` - metrics collection, rule validation

### 7. FIPS-specific testing patterns
For Go FIPS packages, use the `test/go-fips-check` pipeline:
```yaml
test:
  pipeline:
    - uses: test/go-fips-check
    - runs: |
        # Functional tests go here
```

For Java FIPS packages, use the `java-fips/algorithms` pipeline:
```yaml
test:
  pipeline:
    - uses: java-fips/algorithms
      with:
        java-version: ${{vars.java-version}}
    - runs: |
        # Functional tests go here
```
