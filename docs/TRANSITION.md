# Transitions

`stereo transition` helps manage shared library transitions for melange packages by analyzing dependencies and determining rebuild requirements.

```bash
stereo transition icu
```

The transition command analyzes a melange package (typically one with a `-dev` subpackage) to:
- Identify shared libraries provided by the package
- Generate regex patterns for matching dependent packages
- Find all packages that depend on the transitioning shared libraries
- Determine build order based on inter-package dependencies
- Generate a comprehensive transition plan

The command outputs a detailed JSON file (`transition/<package>.json`) containing:
- Target package information
- Shared library patterns and versions
- List of packages requiring rebuilds with affected packages
- List of packages already completed (using current shared library versions)
- Ordered build sequence to handle dependencies correctly

**Usage:**
```bash
stereo transition <package-name> [flags]
```

**Flags:**
- `--arch`: Architecture to evaluate (default: x86_64)
- `--extra-repo`: Additional APK repository URLs to include in analysis (can be specified multiple times)

**Examples:**

Basic usage for a library transition:
```bash
stereo transition openssl
```

Include additional repositories for analysis:
```bash
stereo transition icu --extra-repo https://example.com/apk/repo1 --extra-repo https://example.com/apk/repo2
```

The transition command is particularly useful for coordinated updates of shared libraries where multiple dependent packages need to be rebuilt in the correct dependency order. The JSON output includes both packages that need rebuilding and those that are already complete, making it easy to track progress during a transition.
