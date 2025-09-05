# Stereo Archive Workflow

This document describes the Makefile targets for running the stereo archive process workflow.

## Prerequisites

1. **Terraform Plan Files** (for image dependencies):
   - `public-images.tfplan.json` - Public images terraform plan
   - `private-images.tfplan.json` - Private images terraform plan (optional)

2. **Optional Files**:
   - `archive-seeds.json` - Manual seed packages configuration
   - `package-version-metadata/` - Directory containing version stream configurations for multi-version packages (e.g., postgresql-14, postgresql-15, postgresql-16)

## Quick Start

Run the complete archive workflow:
```bash
make archive-workflow-full
```

## Step-by-Step Process

### 1. Resolve Dependencies
```bash
# Resolve all dependencies (build, image, VM, seed, version-streams)
make deps-resolve

# Or run individual steps:
make deps-build           # Build dependencies
make deps-image           # Image dependencies (requires tfplan.json files)
make deps-vm              # VM dependencies
make deps-seed            # Seed dependencies (optional)
make deps-version-streams # Version stream dependencies (optional)
```

### 2. Archive Analysis
```bash
# Generate archive analysis only (365 days retention)
make archive

# Generate archive analysis + withdrawn-packages.txt files (365 days retention)
make archive-generate
```

### 3. Create Modified Indexes
```bash
# Create withdrawn APKINDEX files
make withdraw
```

### 4. Validate Changes
```bash
# Test dependencies with withdrawn packages
make validate-withdrawn
```

## Individual Targets

| Target | Description |
|--------|-------------|
| `deps-resolve` | Run all dependency resolution steps |
| `deps-build` | Resolve build dependencies |
| `deps-image` | Resolve image dependencies from tfplan.json files |
| `deps-vm` | Resolve VM dependencies |
| `deps-seed` | Resolve seed dependencies (optional) |
| `deps-version-streams` | Resolve version stream dependencies (optional) |
| `archive` | Run archive analysis (365 day threshold) |
| `archive-generate` | Run archive analysis + generate withdrawn-packages.txt |
| `withdraw` | Create modified APKINDEX files with packages removed |
| `validate-withdrawn` | Test all dependencies with withdrawn packages |
| `archive-workflow-full` | Run complete workflow: deps → archive → withdraw → validate |
| `clean-archive` | Clean all archive-related output files |

## Expected File Structure

```
stereo/
├── public-images.tfplan.json      # Public images terraform plan
├── private-images.tfplan.json     # Private images terraform plan (optional)
├── archive-seeds.json             # Manual seed packages (optional)
├── package-version-metadata/      # Version stream configurations (optional)
├── resolved/                      # Dependency resolution outputs
│   ├── build/{arch}/              # Build dependencies
│   ├── images/{arch}/             # Image dependencies
│   ├── vms/{arch}/                # VM dependencies
│   ├── seeds/{arch}/              # Seed dependencies
│   └── version-streams/{arch}/    # Version stream dependencies
├── archive/                       # Archive candidates
├── retain/                        # Retained packages
├── withdrawn-packages.txt files   # Per repository (os/, extra-packages/, etc.)
├── withdrawn-indexes/             # Modified APKINDEX files
│   ├── os/{arch}/
│   ├── extra-packages/{arch}/
│   └── enterprise-packages/{arch}/
└── withdrawn-test/                # Validation test results
    ├── resolved/
    └── unresolved/
```

## Error Handling

- Missing tfplan.json files: Warnings logged, but workflow continues
- Missing archive-seeds.json: Info logged, seed step skipped
- Missing package-version-metadata/: Info logged, version-streams step skipped

## Cleanup

```bash
# Clean only archive-related files
make clean-archive

# Clean everything (archive + package builds)
make clean
```

## Architecture Support

All commands support multi-architecture processing (x86_64 and aarch64) by default. Use `--arch` flag with stereo commands directly for single-architecture processing.