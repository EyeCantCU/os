# Authentik Enterprise Patch Automation

A Claude Code skill that automates the process of updating authentik package versions and regenerating enterprise module removal patches.

## Overview

Authentik's upstream repository includes enterprise features that are loaded unconditionally despite licensing restrictions. The stereo repository maintains OSS builds by applying patches that remove all `authentik.enterprise` module references.

This skill automates the tedious manual work required for each version update:
- ✅ Automatically finds enterprise references using ripgrep
- ✅ Compares findings with existing patches
- ✅ Identifies new files needing patches and removed files
- ✅ Generates patch templates with context
- ✅ Validates patches apply cleanly
- ✅ Updates package YAML files with correct version/commit
- ✅ Guides through build and test process

**Why this is needed**: Per [upstream discussion #18682](https://github.com/goauthentik/authentik/discussions/18682), authentik's enterprise code is loaded unconditionally and this situation is unlikely to change.

## Prerequisites

### Required Tools

- **git**: Version control
- **ripgrep (rg)**: Fast code search
  ```bash
  brew install ripgrep  # macOS
  ```
- **uv**: Python package runner
  ```bash
  curl -LsSf https://astral.sh/uv/install.sh | sh
  ```

### Required Environment

- **Working directory**: Must run from the stereo repository root (the directory containing `enterprise-packages/`)
  ```bash
  cd /path/to/stereo
  ```

- **Python dependencies**: Automatically installed by wrapper script
  - `mcp` - Model Context Protocol server
  - `pyyaml` - YAML parsing
  - `ruamel.yaml` - YAML editing (preserves comments)

## Quick Start

### Interactive Mode

```bash
cd /path/to/stereo
./.claude/skills/authentik-patch/run-skill.sh
```

When Claude launches, invoke the skill:
```
/authentik-patch
```

### Direct Mode (Specify Version)

```bash
cd /path/to/stereo
./.claude/skills/authentik-patch/run-skill.sh 2025.12.3
```

Claude will launch and immediately start the update process for version 2025.12.3.

## How It Works

### File Structure

```
authentik-patch/
├── SKILL.md                 # Detailed workflow documentation (what Claude follows)
├── server.py                # MCP server with 8 automation tools
├── run-skill.sh             # Wrapper script (entry point)
├── requirements.txt         # Python dependencies
├── README.md                # This file
└── references/
    └── patch-patterns.md    # 14 documented enterprise patching patterns
```

### MCP Tools

The skill provides 8 specialized tools accessible during the workflow:

1. **checkout_authentik** - Clone and checkout specific version
2. **list_upstream_versions** - Show recent available versions
3. **find_enterprise_refs** - Search for enterprise module references
4. **analyze_patch_coverage** - Compare with existing patches
5. **generate_patch_template** - Create patch templates with context
6. **validate_patches** - Test if patches apply cleanly
7. **update_package_yaml** - Update version/commit in package files
8. **cleanup** - Remove temporary directories

### 5-Phase Workflow

**Phase 1: Gather Information**
- Determine target version
- Check current state
- Select packages to update (authentik, authentik-fips, or both)

**Phase 2: Checkout and Analyze**
- Clone upstream authentik at target version
- Find all enterprise references with ripgrep
- Compare with existing patches
- Identify new/removed/covered files

**Phase 3: Patch Review and Updates**
- Generate templates for new files
- Update existing patches if needed
- Validate all patches apply cleanly

**Phase 4: Update Package Files**
- Update authentik.yaml with new version/commit
- Update authentik-fips.yaml if selected
- Optionally update patch file list

**Phase 5: Build, Test, and Cleanup**
- Lint YAML files
- Build package with `make package/authentik`
- Run tests with `make test/authentik`
- Create commit
- Clean up temporary repositories

## Expected Duration

| Update Type | Estimated Time |
|-------------|---------------|
| Patch version (e.g., 2025.12.1 → 2025.12.2) | 10-15 minutes |
| Minor version (e.g., 2025.11.x → 2025.12.x) | 20-30 minutes |
| Major version or significant changes | 45-60 minutes |

*Note: Times include build and test phases*

## Example Session

```bash
$ cd /path/to/stereo
$ ./.claude/skills/authentik-patch/run-skill.sh 2025.12.3

Starting authentik-patch skill...
MCP server: /path/to/stereo/.claude/skills/authentik-patch/server.py
Working directory: /path/to/stereo
Registering MCP server...
Launching Claude with version: 2025.12.3

# Claude starts and runs the skill...
# Phase 1: Checks current version (2025.12.1), confirms update to 2025.12.3
# Phase 2: Checks out upstream, finds 15 files with enterprise refs
#          Analysis shows 3 new files, 1 removed file, 12 covered
# Phase 3: Generates templates for 3 new files
#          User manually creates patches following patterns
#          Validates all patches apply cleanly
# Phase 4: Updates authentik.yaml and authentik-fips.yaml
# Phase 5: Builds successfully, tests pass, commits changes

✓ authentik updated to 2025.12.3
✓ All patches validated
✓ Build successful
✓ Tests passed
✓ Committed to git

Cleaning up MCP server registration...
```

## Troubleshooting

### "ERROR: Must be run from enterprise-packages repository root"

**Solution**: Navigate to the stereo repository root (the directory containing `enterprise-packages/`)
```bash
cd /path/to/stereo
pwd  # Verify you're in the right place
ls enterprise-packages/  # Should list authentik, authentik-fips, etc.
```

### "ERROR: ripgrep (rg) is not installed"

**Solution**: Install ripgrep
```bash
brew install ripgrep  # macOS
sudo apt install ripgrep  # Ubuntu/Debian
```

### "ERROR: uv is not installed"

**Solution**: Install uv
```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

### Patches Don't Apply

**Symptoms**: `validate_patches` shows failures

**Solutions**:
1. Review error messages to identify which patches fail
2. Check if upstream file structure changed
3. Use `generate_patch_template` to see current state
4. Manually adjust patch file with correct line numbers
5. See `references/patch-patterns.md` for pattern guidance

### Build Fails

**Symptoms**: `make package/authentik` errors

**Common Causes**:
1. **Missed enterprise reference**: Error mentions missing import
   - Go back to Phase 2, re-run `find_enterprise_refs`
   - Add missing reference to appropriate patch

2. **Patch application failed**: Build can't apply a patch
   - Validate patches with `validate_patches` tool
   - Fix failing patch files

3. **Dependency changes**: Upstream changed requirements
   - Update `dependencies.runtime` in package YAML
   - Check upstream `pyproject.toml` for changes

### Tests Fail

**Symptoms**: `make test/authentik` shows failures

**Common Causes**:
1. **Import errors**: Runtime import of enterprise module
   - Check test output for specific import
   - Add to enterprise.patch

2. **Type errors**: kwargs unpacking fails
   - Add defensive coding (Pattern 8.1): `**(value or {})`
   - Update middleware-m2m-fix.patch

3. **Test assertions**: Tests expect enterprise features
   - Comment out enterprise-specific assertions
   - See Pattern 5.1 in patch-patterns.md

### MCP Server Won't Start

**Test Manually**:
```bash
cd /path/to/stereo
uv run --with mcp --with pyyaml --with ruamel.yaml \
    .claude/skills/authentik-patch/server.py
```

If this fails, check:
- Python dependencies installed
- server.py has no syntax errors
- File permissions are correct

## Testing

### Test MCP Server Standalone

```bash
cd /path/to/stereo
uv run --with-requirements .claude/skills/authentik-patch/requirements.txt \
    --with mcp --with pyyaml --with ruamel.yaml \
    .claude/skills/authentik-patch/server.py
```

Should start without errors and wait for MCP messages.

### Test Individual Tools

Use Claude CLI with MCP server registered:
```bash
# List available versions
claude "Use authentik-patch's list_upstream_versions tool"

# Checkout a version
claude "Use authentik-patch's checkout_authentik tool with version 2025.12.3"
```

### Test Full Workflow

Run through complete workflow with a test version update:
```bash
./.claude/skills/authentik-patch/run-skill.sh 2025.12.3
```

## Advanced Usage

### Update Patches Only (No Version Change)

If you need to update patches but keep the same version:

1. Run Phases 1-3 normally
2. Skip Phase 4 (don't update YAML)
3. Run Phase 5 to build/test/commit just the patches

### Add New Patch File

To create a brand new patch file:

1. Create the patch in `enterprise-packages/authentik/`
2. In Phase 4, include it in the patches array
3. Ensure correct ordering (dependencies first)

### Compare Version Differences

```bash
# In the checked-out authentik repo
cd /tmp/authentik-patch-xxx/authentik
git log --oneline version/2025.12.1..version/2025.12.3
git diff version/2025.12.1 version/2025.12.3 -- authentik/
```

## Pattern Reference Quick Links

See `references/patch-patterns.md` for detailed documentation of 14 common patterns:

1. Comment out module imports
2. Comment out multi-line enterprise imports
3. Replace license validation with None
4. Comment out model exclusions
5. Remove ConditionalInheritance calls
6. Create empty stub classes
7. Comment out enterprise test assertions
8. Remove enterprise from Django app list
9. Comment out enterprise provider API calls (frontend)
10. Add None-safe unpacking
11. Comment out serializer mixin inheritance
12. Comment out enterprise search field imports
13. Comment out enterprise device/endpoint imports
14. Comment out SSF stream event imports

## Integration with Stereo Repository

This skill follows stereo repository conventions:

- **Commit format**: `authentik: update to {version}`
- **YAML formatting**: Uses `ruamel.yaml` to preserve comments
- **Linting**: Reminds to run `./lint.sh` on YAML files
- **Testing**: Integrates with `make package/authentik` and `make test/authentik`
- **Guardrails**: Follows patterns from the repo's `CLAUDE.md`

## Contributing

### Adding New Patterns

If you discover new enterprise patching patterns:

1. Document in `references/patch-patterns.md`
2. Include: Before/after code, file types, explanation
3. Update pattern count in documentation

### Improving the Skill

Files to modify:
- **server.py**: Add new MCP tools
- **SKILL.md**: Update workflow phases
- **patch-patterns.md**: Add new patterns
- **README.md**: Update documentation

## Support

### Issues

Report issues or feature requests:
- GitHub: https://github.com/anthropics/claude-code/issues
- Mention: `authentik-patch skill`

### References

- **Upstream discussion**: https://github.com/goauthentik/authentik/discussions/18682
- **Stereo repo conventions**: `CLAUDE.md` (in the repo root)
- **Current patches**: `enterprise-packages/authentik/*.patch`
- **Package definitions**: `enterprise-packages/authentik.yaml`, `enterprise-packages/authentik-fips.yaml`

## License

This skill is part of the stereo enterprise-packages repository tooling.
