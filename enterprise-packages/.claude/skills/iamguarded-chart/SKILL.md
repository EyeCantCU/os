# Build IAMGuarded Chart Packages

Automates creation of IAMGuarded chart APK packages in enterprise-packages.

## What This Skill Does

Creates chart APK packages (chart-iamguarded-{name}) from the iamguarded-charts repository.

**NOT INCLUDED**: IAMGuarded compat subpackages for dependent images - use the separate `iamguarded-package` skill for that.

## Prerequisites

- Access to enterprise-packages and iamguarded-charts repositories
- GitHub CLI authenticated (`gh auth login`)
- Working directory: enterprise-packages root

## Process

### Phase 1: Gather Requirements

Ask the user:
1. **Chart name** (e.g., "mysql", "postgresql", "rabbitmq")

Verify the chart doesn't already exist at the root of enterprise-packages with name chart-iamguarded-CHART_NAME.yaml. If it does, report this to the user and ask them what to do.

### Phase 2: Clone the iamguarded-charts repository

Clone the chart using the MCP tool:

Use the `checkout_iamguarded_charts` tool to clone the iamguarded-charts repository to a temporary directory. Store the returned path in a variable (REPO).

Display the repository path to the user.

### Phase 3: Create Chart Package

Set up directories and authentication:

```bash
# Create chart directory and set up GitHub auth
mkdir -p "./chart-iamguarded-${CHART_NAME}"
gh auth token > "./chart-iamguarded-${CHART_NAME}/.github-token"
```

Create chart package YAML using the MCP tool:

Use the `generate_chart` tool with the following parameters:
- `repo_dir`: The path returned from Phase 2 (REPO variable)
- `chart_name`: The chart name from Phase 1
- `template_path`: ".claude/skills/iamguarded-chart/chart-template.tmpl"
- `output_path`: "./chart-iamguarded-${CHART_NAME}.yaml"

The output should be written to the top of enterprise-packages.

After generating the chart, insert newlines as the chart-template.tmpl has.

### Phase 4: Generate Patch File

Generate a patch following this approach:

1. Open `$REPO/bitnami/$CHART_NAME/templates/NOTES.txt` in editor
2. Make these changes:
   - Remove enterprise advertisements: "Did you know there are enterprise versions of the Bitnami catalog?..."
   - Remove these template functions: `common.warnings.*`, `common.errors.insecureImages`
   - Replace the OCI registry path `oci://registry-1.docker.io/bitnamicharts/[CHART_NAME]` with `cgr.dev/ORGANIZATION/[CHART_NAME]`, preserving the original chart name in the place of CHART_NAME.
   - Replace to "Bitnami" or "bitnami" with "iamguarded".
   - Promotional/enterprise content
3. Generate patch. Write the patch to the enterprise-packages repository to enterprise-packages/chart-iamguarded-[CHART_NAME]/iamguarded.patch. Replace [CHART_NAME] with the name of the chart being packaged:
   ```bash
   cd "$REPO"
   git diff > "$PATCH_FILE"
   ```
Note that you can look at existing patches in enterprise-packages/chart-iamguarded-*/iamguarded.patch to see how they are done as a reference.

Preview the patch and ask user to confirm.

### Phase 5: Build and Test Chart
```bash
# Build the chart package
make package/chart-iamguarded-${CHART_NAME}

# Test the chart package
make test/chart-iamguarded-${CHART_NAME}
```

Note that if the user isn't authenticated, the make package and make test commands will return an SSO URL for the user to click on and login with. If that occurs, report this URL to the user to have them authenticate.

If build fails, analyze the error and fix:
- Missing dependencies → Add to `environment.contents.packages`
- Auth issues → Verify `.github-token` file
- Patch issues → Understand what is missing, make additional changes to the iamguarded-charts repo, and update the patch

If there is a chart that is a dependency of the chart that you are currently building, repeat the chart creation process for that chart, build it, then come back to building this chart.

### Phase 6: Cleanup and Summary

Clean up temporary repository using the MCP tool:

Use the `cleanup` tool with the temporary repository path from Phase 2 (REPO variable) to remove the cloned directory.

Provide summary report:

```
✅ Chart Package Created
   - Name: chart-iamguarded-${CHART_NAME}
   - Version: ${VERSION}
   - Dependencies: [list]
   - Image Dependencies: [list]

📋 Next Steps
   1. Review changes and commit to enterprise-packages
   2. Create PR for chart package
   3. Use `/iamguarded-package` skill to create IAMGuarded compat subpackages
   4. After PR merge, proceed to images-private setup

⚠️  Notes
   - [Any manual steps taken]
   - [Any deviations from standard process]
```

## Chart Dependency Mapping

Map upstream dependencies to IAMGuarded packages. Here are some example dependencies that may be used.
- `common` → `chart-iamguarded-common`
- `minio` → `chart-iamguarded-minio`
- `postgresql` → `chart-iamguarded-postgresql`

Find others in enterprise-packages root: `ls chart-iamguarded-*.yaml`

## Validation Checklist

Before completing:
- [ ] Chart APK builds and tests pass
- [ ] Patch only removes Bitnami/warning references
- [ ] GitHub auth configured (`.github-token` file)
- [ ] All chart dependencies mapped correctly

## Troubleshooting

**"No tags found matching pattern"**
- Check chart name spelling
- Verify chart exists in iamguarded-charts repository

**"Chart path does not exist"**
- Note that within iamguarded-charts, the subdirectory is bitnami, and not iamguarded (iamguarded-charts/bitnami, instead of iamguarded-charts/iamguarded). Within the chart you are defining, the `uses` should still be `iamguarded/...`. The bitnami path is simply for browsing iamguarded-charts.
- Chart may use different directory structure
- Check actual path in cloned repository

**Build fails with missing dependencies**
- Add missing packages to `environment.contents.packages`
- Check if dependency is a chart dependency or runtime dependency

**Patch application fails**
- Review patch content
- May need to manually edit NOTES.txt
- Some charts may not have warnings to remove
