---
name: iamguarded-package
description: Create IAMGuarded compat subpackages for images in enterprise-packages. Use after creating chart packages with iamguarded-chart skill.
---

# Build IAMGuarded Compat Packages

Automates creation of IAMGuarded compat subpackages for images in enterprise-packages.

## What This Skill Does

Creates IAMGuarded compat subpackages for dependent images that are referenced by IAMGuarded charts.

**Prerequisites**: The chart package should already be created using the `/iamguarded-chart` skill.

## Prerequisites

- Access to enterprise-packages and iamguarded-tools repositories
- Working directory: enterprise-packages root
- Chart package already created (use `/iamguarded-chart` skill first)
- MCP server tools available (automatically configured when using `run-skill.sh` wrapper)

## How to Run

**Recommended**: Use the wrapper script which manages the MCP server lifecycle:

**Option 1: Interactive mode**
```bash
./.claude/skills/iamguarded-package/run-skill.sh
```
Then invoke this skill with `/iamguarded-package` when Claude launches.

**Option 2: Direct invocation**
```bash
./.claude/skills/iamguarded-package/run-skill.sh <application-name>
```
Claude will launch with an initial prompt to run this skill for the specified application.

## Process

### Phase 1: Gather Requirements

If not already provided, ask the user:
1. What application are they adding a iamguarded compat package to?

Note: If the skill was invoked via `run-skill.sh <application-name>`, the application name will be in the initial prompt.

Checkout the iamguarded-charts, iamguarded-containers and iamguarded-tools repositories for future reference using the MCP tools:

Use the `checkout_repo` tool three times to clone the required repositories:
- Clone iamguarded-charts
- Clone iamguarded-containers
- Clone iamguarded-tools

Store the returned paths for use in later phases.

### Phase 2: Create IAMGuarded Compat Configs

For each image dependency, do the following two steps:

Step 1: Get the version streams
- In the cloned `iamguarded-containers` repo (CONTAINER_REPO), check in `/bitnami/` for the application and look for versioned subfolders.
- These subfolders define the upstream version streams. When the version has a subfix (for example, "/debian-12"), remove the suffix.
- For example, `./bitnami/argo-cd` has the sub-folders `3.0, 3.1, and 3.2/debian-12`. In that case, the version streams are 3.0, 3.1, and 3.2.

Step 2: Create a config.sh for each version stream using new-config.sh:

```bash
# Create compat config for each version stream
# Example: For mysql with versions "8.0,8.4"
cd "$TOOLS_REPO"
./compat/new-config.sh -n "mysql" -v "8.0,8.4"
```

This creates: `compat/packages/$IMAGE_NAME/$VERSION/config.sh` for each version.


### Phase 3: Add IAMGuarded Compat Subpackages

For each image dependency:

1. **Find the base package** in enterprise-packages:
   Use the `find_package` tool with the image name

2. **Check for existing bitnami-compat subpackage**:
   Use the `has_bitnami_compat` tool with the image name (returns "true" or "false")

3. **Create or update the iamguarded compat package**:

   **If bitnami-compat exists, transform it using these rules**:
   - Copy the bitnami-compat subpackage structure
   - Rename: `${IMAGE_NAME}-bitnami-compat` → `${IMAGE_NAME}-iamguarded-compat`
   - Transform paths:
     - `${{targets.contextdir}}/opt` → `/opt`
     - `${{targets.contextdir}}/opt/bitnami` → `/opt/iamguarded`
     - `${{targets.contextdir}}/usr/bin` → (no change)
   - Update `provides:` to use `-iamguarded-compat`
   - Add appropriate version variables based on `version-path`
   - Replace pipeline steps with:
     - `iamguarded/build-compat`
     - `iamguarded/finalize-compat`
   - Add test pipeline with `iamguarded/test-compat`

   **If bitnami-compat doesn't exist**, create from scratch

    - To create this stage, referencing the relevant Dockerfile (from `iamguarded-containers`) and associated setup scripts can help understand where the paths, files, and links should be created.
    - Additionally, check the associated helm chart in iamguarded-charts to see if the entrypoint or command is being overriden in the final application deployment.
    - A more direct approach is analyzing the upstream image content to understand the file layout. While bitnami has pulled many images from dockerhub, there are still archives of their images, such as [bitnami legacy](https://hub.docker.com/u/bitnamilegacy), which can be used as reference.
      - Use the `analyze_image_paths` MCP tool to examine the image layout:
        - The tool analyzes the image using dive and returns filtered paths from relevant directories: `/opt`, `/usr/bin`, `/etc/`, `/usr/local/`, `/bin`, `/sbin`, `/lib`, `/usr/lib`, `/lib64`, and `/var/run`
        - Example: `analyze_image_paths(image="bitnamilegacy/zookeeper:latest")`
        - The tool returns a JSON structure with file paths, permissions, and other metadata for the filtered directories
        - Note: if the analyze step fails because it can't find the image, ask the user to provide the image to analyze.
    - In most cases, file and folder paths will be created under /opt/bitnami/, with links pointing to/from this directory. In the case of the run stage of compat package, we want to put files in /opt/iamguarded, /iamguarded, and the like.
    - We must also be aware to create links or copy configuration in other directories as well, as is found to be required by the application being packaged and the helm chart which runs it.

4. **Review the changes**:
   Use the `show_package_diff` tool with the package filename (e.g., "mysql.yaml")

Show the user the changes and ask for confirmation before proceeding.

### Phase 4: Build and Test Compat Packages

For each modified package:

First, setup auth to github so the iamguarded-compat pipeline can run:

```bash
gh auth token > ./${IMAGE_NAME}/.github-token
```

Then build and test with:

```bash
# Build the package
make package/${IMAGE_NAME}

# Test the package
make test/${IMAGE_NAME}
```

If build fails, common issues:
- Missing dependencies → Add to runtime deps
- Script references to "bitnami" → May need custom `build-compat.sh` in iamguarded-tools
- Permission issues → Check file modes in finalize step

### Phase 5: Summary and Commit

Show the user the path to the cloned iamguarded-tools directory, and remind them to commit the changes if they wish to keep them.

Next, provide a summary report:

```
✅ IAMGuarded Compat Packages Modified
   - ${IMAGE_1}-iamguarded-compat (versions: X.Y, X.Z)
   - ${IMAGE_2}-iamguarded-compat (versions: A.B)

📋 Next Steps
   1. Review changes and commit to enterprise-packages
   2. Push compat configs to iamguarded-tools repository
   3. Create PR for package publication
   4. After PR merge, proceed to images-private setup:
      - Create images/${IMAGE}-iamguarded/ directories
      - Set up Terraform configuration

⚠️  Notes
   - [Any manual steps taken]
   - [Any deviations from standard process]
```

### Cleanup

Confirm if the user wants to run the clean up job to clean up the temporary directories.

## Version Variable Guidelines

Determine the version variable based on the package's `version-path`:

- **`version-path: 2/debian-12`** → Use major version:
  ```yaml
  var-transforms:
    - from: ${{package.version}}
      match: ^(\d+)\.\d+\.\d+$
      replace: "$1"
      to: major-version
  ```

- **`version-path: 1.29/debian-12`** → Use major.minor:
  ```yaml
  var-transforms:
    - from: ${{package.version}}
      match: ^(\d+\.\d+)\.\d+$
      replace: "$1"
      to: major-minor-version
  ```

## Pipeline Steps Template

Standard pipeline for IAMGuarded compat subpackages:

```yaml
subpackages:
  - name: ${{package.name}}-iamguarded-compat
    dependencies:
      runtime:
        - ${{package.name}}
    options:
      no-provides: true
    pipeline:
      - uses: iamguarded/build-compat
        with:
          package: $IMAGE_NAME
          version: ${{vars.major-version}}  # or major-minor-version

      - uses: iamguarded/finalize-compat
        with:
          package: $IMAGE_NAME
          version: ${{vars.major-version}}  # or major-minor-version

    test:
      pipeline:
        - uses: iamguarded/test-compat
          with:
            package: $IMAGE_NAME
            version: ${{vars.major-version}}  # or major-minor-version
```

## Special Cases

**Complex images**: Some may require custom `build-compat.sh` in iamguarded-tools to transform file contents (remove forbidden words, etc.). When building and testing the package, one of the test pipelines will fail with an error around a word or phrase that is not allowed. Removing of this word should be done via the build-compat.sh script.

**EOL versions**: Add `IAMGUARDED_COMPAT_COMMIT` in config.sh to pin to specific commit.

**No bitnami-compat exists**: Create from scratch following similar packages as examples. Some good examples are in airflow-2.yaml, mysql-9.4.yaml, and argo-cd-3.1.yaml.

## Validation Checklist

Before completing:
- [ ] All compat packages build and tests pass
- [ ] Version variables correctly configured
- [ ] All paths use correct transformations
- [ ] Compat configs created in iamguarded-tools
- [ ] Pipeline steps use correct version variable

## Troubleshooting

**Package not found**
- Verify package YAML exists in enterprise-packages
- Check package naming conventions

**Build fails with missing dependencies**
- Add missing packages to runtime dependencies
- Check if dependency is available in enterprise-packages

**Compat build fails with script errors**
- Create `build-compat.sh` in iamguarded-tools
- Add sed commands to fix problematic references
- See existing `build-compat.sh` files for examples

**Test failures**
- Check that version variable matches the one used in iamguarded-tools config
- Verify file permissions in finalized package
- Review test output for specific errors
