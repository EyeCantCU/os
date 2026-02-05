# Claude Code Knowledge Base

This document provides guidance to Claude Code (claude.ai/code) when working with this repository. It contains learnings, best practices, and guidelines for packaging, testing, and contributing.

## Commit Guidelines
- Never commit directly to the main branch; always create a feature branch
- Each commit should fix one issue or update one package
- Format: `<package-name>: <concise description of change>`
- For version updates: `<package-name>/<version> package update`
- Keep the first line under 72 characters
- Use the imperative mood ("Add feature" not "Added feature")
- Describe what changed and why, not how
- When fixing build failures, explain the cause of the failure and the solution
- For multiple related packages, separate with commas: `pkg1, pkg2: <description>`
- Do not include "Co-Authored-By" lines unless specifically requested

## Build Commands
- Set up QEMU runner (first time): `make fetch-kernel` (only needed once to download kernel files)
- Enable QEMU environment: `export QEMU_KERNEL_IMAGE=$(pwd)/kernel/boot/vmlinuz; export MELANGE_OPTS="--runner=qemu"`
- Build a package: `make package/<package-name>`
- Build with Docker (fallback): `make docker-package/<package-name>`
- Test a package: `make test/<package-name>`
- Debug test failures: `make test-debug/<package-name>` (requires a TTY)
- Lint YAML files: `./lint.sh [filename.yaml]`
- Run in dev container: `make dev-container`
- Scan for vulnerabilities: `wolfictl scan ./packages/$(uname -m)/<package-name-and-version>.apk`
- Explore APK contents: `tar tzv -f packages/$(uname -m)/<package-name-and-version>.apk`

## Code Style Guidelines
- Package YAML files follow strict formatting (enforced by `yam`)
- YAML fields: maintain alphabetical order when possible
- Remove all trailing whitespace from files
- Ensure consistent indentation (2 spaces for YAML)
- Package versioning: increment "epoch" when changing a package without version bump
- Reset "epoch" to 0 for new package versions
- PR naming: `<package-name>/<version>: <description>`
- Package updates may contain an `update:` section for automation
- When patching CVEs: use `<CVE-ID>.patch` naming convention
- Security fixes must be recorded in the advisories repo
- Version streams: use version string in package name, provide logical unversioned forms
- ALWAYS run `./lint.sh <filename.yaml>` after updating any YAML file to ensure proper formatting
- Avoid removing comments unless they are no longer accurate.

## File Structure & Packages

Package definitions are YAML files with build instructions for Melange. Built apk packages end up in the packages/ subdirectory, with an APKINDEX for each architecture. The file structure inside these can be examined with the `tar` command.

Packages in this repository are used to build container images. When packaging a piece of software, look at existing Dockerfiles in the repository for the piece of software you're packaging to understand what files need to be packaged and how they should be laid out.

Split documentation into a separate subpackage where possible. Try to reuse existing melange pipelines where possible.

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

**Never use merged-* packages:**
The `merged-*` packages (`merged-usrsbin`, `merged-bin`, `merged-sbin`, `merged-lib`) are legacy compatibility packages and should **never** be added as build-time or runtime dependencies for new packages or subpackages. These are automatically handled by the base system when needed.

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

### 1. Always use `set -euo pipefail` as a single command
```bash
set -euo pipefail
```
- `-e`: Exit immediately if any command fails
- `-u`: Treat unset variables as an error
- `-o pipefail`: Fail if any command in a pipeline fails (not just the last one)

This ensures tests fail fast and don't mask errors.

**CRITICAL:** Always use `set -euo pipefail` as a single combined command. **NEVER** use separate commands like `set -e` or `set -o pipefail` alone:

```bash
# Good - always use the full combined form
set -euo pipefail

# Bad - never use separate set commands
set -e
set -u
set -o pipefail
```

**IMPORTANT:** Always add `set -euo pipefail` at the start of `post:` blocks in `test/daemon-check-output` to catch pipeline failures and handle unset variables:

```yaml
# Good - full set -euo pipefail
post: |-
  set -euo pipefail
  curl -sf http://localhost:8080/metrics | grep -F "uptime"

# Bad - incomplete, only pipefail
post: |-
  set -o pipefail
  curl -sf http://localhost:8080/metrics | grep -F "uptime"

# Bad - no shell options set, failures may be masked
post: |-
  curl -sf http://localhost:8080/metrics | grep -F "uptime"
```

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

**`test/tw/docs`** - For documentation subpackages:
```yaml
subpackages:
  - name: ${{package.name}}-doc
    test:
      pipeline:
        - uses: test/tw/docs
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

**`test/tw/debugpackage`** - For packages providing debug symbols:
```yaml
test:
  pipeline:
    - uses: test/tw/debugpackage
```
Check that the package structure matches debug package requirements and contains appropriate debug information files.

**`test/tw/help-check`** - For CLI tools:
```yaml
test:
  pipeline:
    - uses: test/tw/help-check
      with:
        bins: ${{package.name}}
        # Optional: verify help output contains specific strings
        expect-contains: "Usage Options"
```
Verifies binaries respond to help flags (--help, -h, etc.) and optionally validates help content.

**`test/tw/ver-check`** - For version validation:
```yaml
test:
  pipeline:
    - uses: test/tw/ver-check
      with:
        bins: ${{package.name}}
        version: ${{package.version}}
```
Verifies binaries report the correct version matching package metadata.

**`test/tw/header-check`** - For C/C++ development packages:
```yaml
subpackages:
  - name: ${{package.name}}-dev
    test:
      pipeline:
        - uses: test/tw/header-check
          # Optional: specify custom compiler flags
          with:
            configure-opts: "-DENABLE_FEATURE_X"
```
Validates that C/C++ header files can be successfully included and compiled.

**`test/tw/devpackage`** - For development packages:
```yaml
subpackages:
  - name: ${{package.name}}-dev
    test:
      pipeline:
        - uses: test/tw/devpackage
```
Validates that a package contains appropriate development files (headers, static libraries, pkg-config files).

**`test/tw/staticpackage`** - For static library packages:
```yaml
subpackages:
  - name: ${{package.name}}-static
    test:
      pipeline:
        - uses: test/tw/staticpackage
```
Validates that a package contains only static libraries (.a files).

**`test/tw/byproductpackage`** - For by-product packages:
```yaml
test:
  pipeline:
    - uses: test/tw/byproductpackage
```
Validates automatically generated packages created during the build process.

**`test/tw/emptypackage`** - For empty packages:
```yaml
test:
  pipeline:
    - uses: test/tw/emptypackage
```
Validates that a package is intentionally empty.

**`test/tw/verify-service`** - For systemd service files:
```yaml
test:
  pipeline:
    - uses: test/tw/verify-service
      # Optional: skip specific files or include doc tests
      with:
        skip-files: "legacy.service"
        man: "true"
```
Validates systemd service files are properly formatted and follow best practices.

**`test/tw/contains-files`** - For verifying file presence:
```yaml
test:
  pipeline:
    - uses: test/tw/contains-files
      with:
        files: |
          /usr/bin/myapp
          /etc/myapp/config.yaml
```
Checks for the presence of specific files or patterns in a package.

**`test/tw/no-docs`** - For packages without documentation:
```yaml
test:
  pipeline:
    - uses: test/tw/no-docs
```
Ensures a package contains no documentation files (useful for runtime-only packages).

**`test/tw/symlink-check`** - For symlink validation:
```yaml
test:
  pipeline:
    - uses: test/tw/symlink-check
      # Optional: allow specific symlink types
      with:
        allow-dangling: false
        allow-absolute: false
```
Verifies all symlinks point to valid targets and meet policy requirements.

**`test/tw/gem-check`** - For Ruby gems:
```yaml
test:
  pipeline:
    - uses: test/tw/gem-check
      # Optional: specify gem name if different from package
      with:
        require: "activesupport"
```
Validates that a Ruby gem can be properly required and loaded.

**`test/tw/pip-check`** - For Python packages:
```yaml
test:
  pipeline:
    - uses: test/tw/pip-check
      # Optional: specify Python version
      with:
        python: python3.11
```
Validates Python package dependencies are correctly installed using pip check.

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

### 8. Use `grep -F` for literal string matching, avoid `-q` flag
Use `grep -F` (fixed string) instead of plain `grep` for literal strings - it's safer and faster.

**IMPORTANT:** Never use `grep -q` (quiet mode) in tests as it suppresses output and makes debugging failures harder:

```bash
# Good - shows what matched
curl -sf http://localhost:8080/metrics | grep -F "otelcol_process_uptime"
netstat -ln | grep -F ":8080"

# Bad - regex can have unexpected matches
curl -sf http://localhost:8080/metrics | grep "otelcol_process_uptime"

# Bad - silences output, makes debugging difficult
curl -sf http://localhost:8080/metrics | grep -qF "otelcol_process_uptime"
```

When tests fail, seeing the actual output helps diagnose issues quickly. The `-q` flag hides this valuable information.

### 9. Use environment variables for test configuration
Centralize port numbers and repeated values using environment variables:

```yaml
# Good
test:
  environment:
    environment:
      GRPC_PORT: "4317"
      METRICS_PORT: "8888"
  pipeline:
    - uses: test/daemon-check-output
      with:
        post: |-
          curl -sf "http://localhost:${METRICS_PORT}/metrics" | grep -F "uptime"
          netstat -ln | grep -F ":${GRPC_PORT}"

# Bad - hardcoded values repeated throughout
post: |-
  curl -sf http://localhost:8888/metrics | grep -F "uptime"
  netstat -ln | grep -F ':4317'
```

### 10. Quote paths in shell scripts
Always quote variable expansions in paths to handle spaces and special characters safely:

```yaml
# Good - quoted paths
pipeline:
  - runs: |
      mkdir -p "${{targets.contextdir}}/${{vars.build_folder}}"
      mv "${{vars.build_folder}}"/* "${{targets.contextdir}}/${{vars.build_folder}}"
      mv ./fips/fips.go "${{targets.contextdir}}/${{vars.build_folder}}"

# Bad - unquoted paths (can break with spaces)
pipeline:
  - runs: |
      mkdir -p ${{targets.contextdir}}/${{vars.build_folder}}
      mv ${{vars.build_folder}}/* ${{targets.contextdir}}/${{vars.build_folder}}
      mv ./fips/fips.go ${{targets.contextdir}}/${{vars.build_folder}}
```

### 11. Use reasonable timeouts for daemon tests
Set appropriate timeout values based on service type:

- **Go binaries**: 30-60 seconds
- **Java applications**: 90-120 seconds
- **Python/Ruby services**: 60-90 seconds
- **Heavy services (databases, etc.)**: 120-180 seconds

```yaml
# Good
- uses: test/daemon-check-output
  with:
    timeout: 60  # Sufficient for Go services

# Bad
- uses: test/daemon-check-output
  with:
    timeout: 150  # Excessive for a lightweight Go binary
```

### 12. Use `setup` parameter for test configuration
Move config file creation to `setup` parameter in `test/daemon-check-output`:

```yaml
# Good - setup separate from execution
- uses: test/daemon-check-output
  with:
    setup: |-
      cat << 'EOF' > /tmp/config.yaml
      receivers:
        otlp:
      EOF
    start: service-name --config=/tmp/config.yaml
    post: |-
      service-name validate --config=/tmp/config.yaml

# Bad - config creation as separate test step
- name: Config validation
  runs: |
    cat << 'EOF' > /tmp/config.yaml
    receivers:
      otlp:
    EOF
    service-name --config=/tmp/config.yaml validate
```

### 13. Use `tee` instead of `cat` for creating config files
When creating config files in tests, use `tee` instead of `cat` to write the file. This shows the config content in the output, making debugging failures much easier:

```bash
# Good - shows config content in output
tee /tmp/config.yaml << 'EOF'
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
EOF

# Bad - silent, no output shown
cat << 'EOF' > /tmp/config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
EOF
```

Benefits of using `tee`:
- Config content appears in test logs, helping diagnose configuration issues
- No need to add separate `cat /tmp/config.yaml` commands for debugging
- Maintains the same functionality as `cat >` while improving observability

**Note:** `tee` writes to both the file and stdout by default. This is desired behavior for test visibility.

### 14. Use `stat` instead of `test -f` for file existence checks
When verifying files exist in tests, use `stat` instead of `test -f` or `test -x`. The `test` builtin only returns exit code 1 on failure with no error message, making it hard to diagnose failures in logs:

```bash
# Bad - silent failure, only shows exit code 1
test -f /usr/share/java/zookeeper/conf/logback.xml
test -x /usr/share/java/zookeeper/bin/zkServer.sh

# Good - shows clear error message on failure
stat /usr/share/java/zookeeper/conf/logback.xml
stat /usr/share/java/zookeeper/bin/zkServer.sh
```

**Output comparison on missing files:**
```bash
# test -f gives no useful output
$ test -f /foo/bar
$ echo $?
1

# stat shows exactly what's wrong
$ stat /foo/bar
stat: cannot statx '/foo/bar': No such file or directory
```

For checking executability specifically, combine `stat` with a follow-up check:
```bash
# Verify file exists and is executable
stat /usr/bin/myapp
test -x /usr/bin/myapp || { echo "ERROR: /usr/bin/myapp is not executable"; exit 1; }
```

## Ruby Package Guidelines

When working with Ruby packages:

- Always increment the epoch when updating a package
- When adding a test, add it to all Ruby version variants (e.g., ruby3.2-*, ruby3.3-*, ruby3.4-*)
- Test environment should include ruby-${{vars.rubyMM}} and any direct dependencies
- Use the test/tw/gem-check pipeline step when appropriate to verify gem installation
- Implement thorough testing for gem functionality, not just loading

### Testing Ruby Packages

#### Basic Testing Structure
```yaml
test:
  environment:
    contents:
      packages:
        - ruby-${{vars.rubyMM}}
  pipeline:
    - uses: test/tw/gem-check  # Verify gem installation
    - name: Verify library loading
      runs: |
        ruby -e "require 'gem_name'; puts 'Successfully loaded gem'"
    - name: Test basic functionality
      runs: |
        ruby <<-EOF
        require 'gem_name'

        begin
          # Actual functionality tests with sample inputs and expected outputs
          # Use raise to fail the test on unexpected results
          puts "All tests passed!"
        rescue => e
          puts "Test failed: \#{e.message}"
          exit 1
        end
        EOF
```

#### Testing Best Practices
- Always read `pipelines/test` to learn what test pipelines are available.
- CRITICAL: Always test the ACTUAL PACKAGE that is being built, not just its dependencies
- Ensure tests exercise the main functionality that users of the gem would use
- Include comprehensive tests that verify actual gem functionality, not just loading
- Test with realistic inputs and verify expected outputs
- Use begin/rescue blocks to handle errors and provide informative failure messages
- Test edge cases and parameter variations where applicable
- For CLI tools, verify command execution (e.g., `rspec --version`)
- Group related tests into logical sections with clear pass/fail messages
- If a gem has optional parameters (like bias, ignore flags), test those too when possible
- Use exit code 1 to indicate test failures
- Validate that key classes, methods, and constants from the package are present and working
- Write tests that verify command behavior, not just execution
- When adding tests, always review existing tests and avoid creating redundant tests.
- When a command is expected to fail, explicitly check for an error code and fail if the command passes.
- Use your understanding of the package under test to determine the core functionality to validate
- All shell code should be compatible with busybox, not bash-specific features
- When including multiple shell commands, organize them into semantic groupings with comments (if logical grouping exists) or sort them alphabetically
- For shell condition checks, use direct comparison with `[ "$OUTPUT" = "expected" ]` style
- The test environment is ephemeral, any non-zero exit code indicates a failure, you do not need to explicitly validate exit codes unless they are relevant to the test
- For numeric outputs, use appropriate numeric comparisons like `[ "$COUNT" -eq 3 ]`
- When matching patterns in complex outputs, use variable expansion with grep but without the error exit: `[ "$(echo "$OUTPUT" | grep "pattern")" != "" ]`

### Avoiding Fragile Tests
- For version checks, simply run `--version` without validating the output at all
- Never validate specific version numbers in tests, even when using `${{package.version}}` interpolation
  - Version number formats may change (e.g., from "1.2" to "v1.2.0")
  - Additional information may be added to version outputs between releases
  - Patch releases may include suffixes or build information that break exact matches
- When testing CLI programs, focus only on successful execution of commands
- For version and help commands, verify:
  - The command runs successfully (non-zero exit code would fail the test naturally)
  - For extremely important elements, check their presence very loosely with pattern matching
- Focus exclusively on testing behavior and functionality, not output format or content
- For help text, at most verify that key commands appear somewhere in the output
- If you must check for output content, use very minimal pattern matching looking for single keywords

#### Common Testing Mistakes to Avoid
- Testing a dependency instead of the actual package being built
- Only testing that a gem can be loaded without testing any functionality
- Missing required dependencies in the test environment
- Testing trivial aspects while ignoring core functionality
- Failing to handle errors or provide useful error messages

#### Dependencies
- For dependencies, check if they actually need to be specified in the test environment or if they are already included via package dependencies
- Common issue: missing gem dependencies often result in loading errors at test time
- Test that expected dependencies are present and correctly loaded

## Debugging

When debugging failed builds, try cloning the source code from the melange file locally to a directory here to study the build system.

## Other General Notes

Follow the previously mentioned notes as a baseline, and then also consider these additional notes:
- Certain packages are a bit weird, like Java and Python. Make sure to check existing examples, but especially note the following:
  - Use py3.x-supported-y packages whenever possible
  - Use a modern and supported JVM/JDK version (making sure to pick the right packages as well) and loading the environment variables properly
  - Avoid using obsolete package versions
- It is worthwhile to search for similar packages to use as working examples over generating something entirely new.
