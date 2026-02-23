# Authentik Enterprise Patch Updates

Automates the process of updating authentik package versions and regenerating enterprise module patches.

## What This Skill Does

Helps maintain OSS builds of authentik by automating:
- Checking out upstream authentik at specific versions
- Finding all authentik.enterprise module references
- Comparing with existing patches to identify changes
- Guiding patch file creation/updates
- Updating package YAML files with new version and commit
- Validating patches apply cleanly

**Problem**: Authentik's upstream code unconditionally loads enterprise components despite licensing restrictions (see [upstream discussion #18682](https://github.com/goauthentik/authentik/discussions/18682)). Every version update requires manually finding and patching enterprise references.

## Prerequisites

- **Working directory**: enterprise-packages repository root (`/Users/matthew.ramirez/work/stereo`)
- **Tools required**: git, ripgrep (rg), uv
- **MCP server**: Automatically configured when using `run-skill.sh` wrapper

## How to Run

**Recommended**: Use the wrapper script which manages the MCP server lifecycle:

**Option 1: Interactive mode**
```bash
./.claude/skills/authentik-patch/run-skill.sh
```
Then invoke this skill with `/authentik-patch` when Claude launches.

**Option 2: Direct invocation with version**
```bash
./.claude/skills/authentik-patch/run-skill.sh 2025.12.3
```
Claude will launch with an initial prompt to update to the specified version.

## Workflow

### Phase 1: Gather Information

**Goal**: Determine target version and current state

**Steps**:

1. **Get target version** from user (or from launch argument)

2. **Check current version**:
   - Read `enterprise-packages/authentik.yaml`
   - Note current version and expected-commit

3. **Show available versions**:
   - Use `list_upstream_versions` tool to show recent releases
   - Confirm target version with user

4. **Determine scope**:
   - Ask which packages to update:
     - `authentik` only
     - `authentik-fips` only
     - Both (recommended)

**Example**:
```
What version would you like to update to?
> 2025.12.3

Current version in authentik.yaml: 2025.12.1

Recent upstream versions:
  - 2025.12.3
  - 2025.12.2
  - 2025.12.1
  - 2025.11.4
  ...

Which packages should be updated?
  1. authentik only
  2. authentik-fips only
  3. Both (recommended)
> 3
```

---

### Phase 2: Checkout and Analyze

**Goal**: Checkout target version and analyze enterprise references

**Steps**:

1. **Checkout authentik**:
   - Use `checkout_authentik` tool with target version
   - Store returned repo path and commit hash
   - Example: `checkout_authentik(version="2025.12.3")`

2. **Find enterprise references**:
   - Use `find_enterprise_refs` tool on the repo path
   - This runs: `rg "authentik\.enterprise" -g '*.py' -g '!authentik/enterprise/**' --json`
   - Results are cached for later phases
   - Example: `find_enterprise_refs(repo_path="/tmp/authentik-patch-xxx/authentik")`

3. **Analyze patch coverage**:
   - Use `analyze_patch_coverage` tool
   - Compares current references with existing patches
   - Identifies:
     - **New files**: Have enterprise refs but not in patches (need new patches)
     - **Removed files**: In patches but no longer have refs (can simplify patches)
     - **Covered files**: Already patched (may need updates)
   - Example: `analyze_patch_coverage(repo_path="/tmp/...", patch_dir="enterprise-packages/authentik/")`

4. **Review findings with user**:
   - Display analysis results
   - Discuss approach for new/changed files

**Example Output**:
```
## Patch Coverage Analysis

Current enterprise references: 15 files
Existing patches cover: 12 files
New files (not in patches): 3
Removed files (in patches but no refs): 1

### New Files Needing Patches:
  - authentik/providers/oauth2/api/tokens.py (2 references)
  - authentik/core/api/permissions.py (1 reference)
  - authentik/stages/email/api.py (1 reference)

### Files No Longer Have Enterprise Refs:
  - authentik/lib/utils/deprecated.py
```

---

### Phase 3: Patch Review and Updates

**Goal**: Update patches for the new version

**Approach**: This phase requires user collaboration to update patch files manually. The skill provides tools to guide the process.

**For New Files** (files with enterprise refs but not in patches):

1. **Generate patch template**:
   - Use `generate_patch_template` tool for each new file
   - Shows enterprise references with context (3 lines before/after)
   - Suggests applicable patterns from `references/patch-patterns.md`
   - Example: `generate_patch_template(file_path="authentik/providers/oauth2/api/tokens.py", repo_path="/tmp/...")`

2. **Create patch file**:
   - User manually creates the patch following suggested patterns
   - Common approach:
     - Make changes in the checked-out repo
     - Generate diff: `cd /tmp/authentik-patch-xxx/authentik && git diff path/to/file.py > /path/to/stereo/enterprise-packages/authentik/new-file.patch`
     - Add patch header (description, author, forwarded: not-needed)
     - Or update existing patch file (e.g., `enterprise.patch`)

**For Covered Files** (files already in patches):

1. **Check if patches still apply**:
   - Use `validate_patches` tool
   - Shows which patches apply cleanly and which fail
   - Example: `validate_patches(repo_path="/tmp/...", patch_dir="enterprise-packages/authentik/")`

2. **Update failing patches**:
   - For each failing patch:
     - Read the patch file to understand what it does
     - Use `generate_patch_template` to see current enterprise references
     - Update the patch file with adjusted line numbers/content
     - Re-validate

**For Removed Files** (no longer have enterprise refs):

1. **Simplify patches**:
   - Review patch files that reference these files
   - Consider removing those sections from patches
   - Validate remaining patches still work

**Validation**:

After updating patches, validate they all apply cleanly:
```
validate_patches(repo_path="/tmp/authentik-patch-xxx/authentik",
                 patch_dir="enterprise-packages/authentik/")
```

All patches must pass validation before proceeding to Phase 4.

**Key Patch Files**:
- `root.settings.patch` - Removes enterprise from Django TENANT_APPS
- `enterprise.patch` - Main patch (11 files, removes imports/references)
- `enterprise.mro.patch` - Fixes Method Resolution Order issues (5 files)
- `frontend-sync-chart.patch` - Removes enterprise UI elements (TypeScript)
- `middleware-m2m-fix.patch` - Adds defensive None handling

**Pattern Reference**:
Refer to `references/patch-patterns.md` for 14 documented patterns with before/after examples.

---

### Phase 4: Update Package Files

**Goal**: Update package YAML files with new version, commit, and patches (if changed)

**Steps**:

1. **Prepare patch list** (if patches changed):
   - List all patch files in order:
     ```
     patches = [
         "root.settings.patch",
         "enterprise.patch",
         "enterprise.mro.patch",
         "frontend-sync-chart.patch",
         "middleware-m2m-fix.patch"
     ]
     ```
   - If new patches were added, include them in the list
   - Order matters: apply in dependency order

2. **Update authentik.yaml**:
   - Use `update_package_yaml` tool
   - Updates:
     - `package.version` → new version
     - `pipeline[git-checkout].expected-commit` → commit hash from Phase 2
     - `pipeline[patch].patches` → patch list (if provided)
   - Example:
     ```
     update_package_yaml(
         package_name="authentik",
         version="2025.12.3",
         commit="abc123def456...",
         patches=patches  # optional
     )
     ```

3. **Update authentik-fips.yaml** (if selected in Phase 1):
   - Same process as authentik.yaml
   - Uses identical patch files
   - Example:
     ```
     update_package_yaml(
         package_name="authentik-fips",
         version="2025.12.3",
         commit="abc123def456...",
         patches=patches  # optional
     )
     ```

4. **Review changes**:
   - Use `show_package_diff` or git diff to review YAML changes
   - Confirm with user before proceeding

**Example**:
```
Updated enterprise-packages/authentik.yaml:
  - version: 2025.12.3
  - commit: dc2332a316e73d1cfaaea23dc117cf0b1af3f95a
  - patches: 5 files

Updated enterprise-packages/authentik-fips.yaml:
  - version: 2025.12.3
  - commit: dc2332a316e73d1cfaaea23dc117cf0b1af3f95a
  - patches: 5 files
```

---

### Phase 5: Build, Test, and Cleanup

**Goal**: Validate the changes and commit

**Steps**:

1. **Lint YAML files**:
   ```bash
   ./lint.sh enterprise-packages/authentik.yaml
   ./lint.sh enterprise-packages/authentik-fips.yaml
   ```
   Fix any formatting issues.

2. **Build package**:
   ```bash
   make package/authentik
   ```
   - Watch for build failures
   - Common issues:
     - Patches don't apply → Go back to Phase 3
     - New enterprise references missed → Go back to Phase 2
     - Build dependencies changed → Update package YAML

3. **Run tests** (if build succeeds):
   ```bash
   make test/authentik
   ```
   - Verify authentik starts correctly
   - Check for import errors or runtime failures
   - Review test output for enterprise-related errors

4. **Create commit**:
   - Follow stereo commit conventions (see `/Users/matthew.ramirez/work/stereo/CLAUDE.md`)
   - Format: `authentik: update to {version}`
   - Example:
     ```bash
     git add enterprise-packages/authentik.yaml \
             enterprise-packages/authentik-fips.yaml \
             enterprise-packages/authentik/*.patch

     git commit -m "$(cat <<'EOF'
     authentik: update to 2025.12.3

     - Updated package version and expected-commit
     - Regenerated enterprise patches for new version
     - Added patch for new OAuth token endpoint

     Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>
     EOF
     )"
     ```

5. **Cleanup**:
   - Use `cleanup` tool to remove temporary authentik checkout
   - Example: `cleanup(temp_dir="/tmp/authentik-patch-xxx")`

6. **Summary**:
   - Report what was updated
   - Note any issues encountered
   - Suggest next steps (e.g., testing in dev environment)

---

## Error Recovery

### Patches Don't Apply

**Symptoms**: `validate_patches` shows failures

**Recovery**:
1. Note which patches fail and the error messages
2. Use `generate_patch_template` to see current file state
3. Manually update patch file with correct line numbers/content
4. Re-validate
5. Common causes:
   - Upstream code structure changed
   - Line numbers shifted due to other changes
   - Enterprise references moved to different files

### Build Fails After Patching

**Symptoms**: `make package/authentik` fails

**Recovery**:
1. Check build output for specific error
2. If import error: Missed an enterprise reference
   - Go back to Phase 2, re-run `find_enterprise_refs`
   - Look for the specific import mentioned in error
   - Add to appropriate patch file
3. If patch application fails during build:
   - Validate patches locally with `validate_patches`
   - Fix failing patches

### Tests Fail

**Symptoms**: `make test/authentik` shows failures

**Recovery**:
1. Review test output for:
   - Import errors → Missing patches
   - Runtime errors → May need defensive coding (Pattern 8.1)
   - Assertion failures → Tests may expect enterprise features
2. Common fixes:
   - Add None-safe unpacking to middleware
   - Comment out enterprise-specific test assertions
   - Ensure all ConditionalInheritance calls are removed

### Missed Enterprise References

**Symptoms**: Runtime errors about missing modules

**Recovery**:
1. Search for the specific module name in the repo
2. Add appropriate patch following documented patterns
3. Rebuild and retest

---

## Tips

- **Start with recent versions**: Fewer changes between minor version bumps
- **Validate often**: Run `validate_patches` after each patch update
- **Use pattern reference**: `references/patch-patterns.md` has 14 common patterns
- **Test incrementally**: Apply patches one at a time if troubleshooting
- **Keep patches organized**: Group related changes in same patch file
- **Document new patterns**: If you find a new pattern, add it to patch-patterns.md

## Common Patterns Quick Reference

1. **Comment out imports**: `#from authentik.enterprise...`
2. **Replace license checks**: → `None`
3. **Remove ConditionalInheritance**: Comment out the entire call
4. **Create stub classes**: `class Mixin: pass`
5. **Defensive unpacking**: `**(value or {})`

See `references/patch-patterns.md` for detailed examples.

## Expected Duration

- **Small update** (patch version, e.g., 2025.12.1 → 2025.12.2): 10-15 minutes
- **Medium update** (minor version, e.g., 2025.11.x → 2025.12.x): 20-30 minutes
- **Large update** (major version or significant upstream changes): 45-60 minutes

## Troubleshooting

### MCP Server Won't Start

```bash
# Test server directly
cd /Users/matthew.ramirez/work/stereo
uv run --with mcp --with pyyaml --with ruamel.yaml \
    .claude/skills/authentik-patch/server.py
```

### Ripgrep Not Found

```bash
# Install ripgrep
brew install ripgrep  # macOS
```

### Working Directory Issues

Ensure you're in the enterprise-packages repository root:
```bash
cd /Users/matthew.ramirez/work/stereo
pwd  # Should show stereo repo path
ls enterprise-packages/authentik.yaml  # Should exist
```

### ruamel.yaml Not Available

The skill will fall back to pyyaml, but may not preserve YAML formatting:
```bash
uv pip install ruamel.yaml
```

## Next Steps After Skill Completes

1. **Review the commit**: `git show HEAD`
2. **Push to branch** (not main):
   ```bash
   git checkout -b authentik-update-2025.12.3
   git push origin authentik-update-2025.12.3
   ```
3. **Create PR** for review
4. **Test in CI/CD** pipeline
5. **Deploy to staging** environment for integration testing

---

## Advanced Usage

### Updating Only Patches (Same Version)

If patches need updates but version stays the same:

1. Skip Phase 1 (use current version)
2. Run Phases 2-3 to update patches
3. Skip Phase 4 (don't update YAML)
4. Run Phase 5 (build, test, commit just the patches)

### Adding New Patch Files

If creating a new patch file (not updating existing):

1. Create the patch file in `enterprise-packages/authentik/`
2. Update patch list in Phase 4 to include new file
3. Ensure proper ordering (dependencies first)

### Comparing Across Versions

To see what changed between versions:
```bash
cd /tmp/authentik-patch-xxx/authentik
git log --oneline version/2025.12.1..version/2025.12.3
git diff version/2025.12.1 version/2025.12.3 -- authentik/
```

---

## References

- **Upstream discussion**: https://github.com/goauthentik/authentik/discussions/18682
- **Stereo CLAUDE.md**: `/Users/matthew.ramirez/work/stereo/CLAUDE.md`
- **Patch patterns**: `references/patch-patterns.md`
- **Current patches**: `enterprise-packages/authentik/*.patch`
- **Package files**: `enterprise-packages/authentik.yaml`, `enterprise-packages/authentik-fips.yaml`
