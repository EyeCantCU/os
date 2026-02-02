# IAMGuarded Package Skill

Claude skill for automating the creation of IAMGuarded compat subpackages for images.


## Overview

This skill automates compat package creation (Part II of the IAMGuarded workflow):
- Creates IAMGuarded compat configs in iamguarded-tools
- Adds IAMGuarded compat subpackages to existing packages
- Transforms bitnami-compat subpackages to iamguarded-compat
- Builds and tests compat packages

**Prerequisites**: The chart package should already be created using the `/iamguarded-chart` skill first.

## Quickstart

Run the skill with the shell script (replace PACKAGE with the package you want to generate generate iamguarded packages for)

```bash
./.claude/skills/iamguarded-package/run-skill.sh PACKAGE
```

## Files

### SKILL.md
The Claude skill definition that provides step-by-step instructions for the LLM. The workflow is organized into 5 phases:

1. **Gather Requirements** - Collect image names and version streams
2. **Create IAMGuarded Compat Configs** - Set up configs in iamguarded-tools
3. **Add IAMGuarded Compat Subpackages** - Modify package YAMLs
4. **Build and Test Compat Packages** - Compile and validate packages
5. **Cleanup and Summary** - Remove temp files and report results

### server.py
Python MCP (Model Context Protocol) server that provides tools for the skill:

**MCP Tools:**
- `checkout_repo` - Clone repositories (iamguarded-tools, iamguarded-containers, iamguarded-charts) to temp directory
- `cleanup` - Remove temporary directory
- `find_package` - Find package YAML file for an image
- `has_bitnami_compat` - Check if package has bitnami-compat subpackage
- `get_version_variable` - Determine version variable type (major-version or major-minor-version)
- `show_package_diff` - Show git diff for a package file

**Features:**
- Maintains state of cloned repositories to avoid re-cloning
- Working directory validation (must be in enterprise-packages root)
- Error handling with clear user and Claude-friendly messages
- YAML parsing using PyYAML
- Git operations for repository management

**Dependencies:**
- `mcp>=1.0.0` - MCP SDK for tool integration
- `pyyaml>=6.0` - YAML parsing
- `uv` - Fast Python package installer and runner

Dependencies are automatically installed by the wrapper script using `uv`.

### run-skill.sh
Wrapper script that manages the MCP server lifecycle:
- Checks for `uv` installation
- Installs Python dependencies automatically
- Registers MCP server in local scope (skill execution only)
- Launches Claude with the skill
- Cleans up MCP registration on exit (via trap)

This ensures the MCP tools are only available during skill execution and don't pollute the global tool namespace.

### scripts/tools.sh
Legacy bash utility script (deprecated in favor of MCP server). Kept for reference but no longer used by the skill.

## Usage

### Prerequisites

1. Install `uv` if not already installed:
   ```bash
   curl -LsSf https://astral.sh/uv/install.sh | sh
   ```

2. Ensure other prerequisites are met:
   - In enterprise-packages repository root
   - Chart package already created (use `/iamguarded-chart` first)
   - Access to iamguarded-tools repo

### Running the Skill

Use the wrapper script (recommended):

**Option 1: Direct invocation with application name**
```bash
./.claude/skills/iamguarded-package/run-skill.sh kafka
```

This automatically starts Claude with a prompt to invoke the skill for the specified application (e.g., "kafka").

**Option 2: Interactive mode**
```bash
./.claude/skills/iamguarded-package/run-skill.sh
```

Then invoke the skill within Claude:
```
/iamguarded-package
```

The script will:
- Install Python dependencies automatically with uv
- Register the MCP server (local scope, skill execution only)
- Launch Claude (optionally with initial prompt for the application)
- Clean up MCP registration on exit

### During Skill Execution

1. The skill will attempt to automatically figure out the relevant information, but may prompt the user in some cases.

2. Follow the prompts and review generated files

3. Commit changes to both enterprise-packages and iamguarded-tools

## Testing

The MCP server can be tested by running it directly:

```bash
# Run the server with uv (it will wait for MCP protocol messages on stdin)
uv run --with mcp --with pyyaml server.py
```

For manual testing of individual functions, you can import the tool functions in a Python shell:

```python
import asyncio
from server import find_package, has_bitnami_compat, get_version_variable

# Test finding a package
result = asyncio.run(find_package("mysql"))
print(result[0].text)

# Test checking for bitnami compat
result = asyncio.run(has_bitnami_compat("mysql"))
print(result[0].text)

# Test getting version variable
result = asyncio.run(get_version_variable("mysql"))
print(result[0].text)
```

Or test the full wrapper script flow:

```bash
# This will set up everything, launch Claude, then clean up
./.claude/skills/iamguarded-package/run-skill.sh
```

## Manual Steps

The skill provides guidance, but some steps may require manual editing:

1. **Custom build scripts**: Some images may need custom `build-compat.sh`
   - Created in iamguarded-tools repository
   - Used to transform files during compat build

## Troubleshooting

See the Troubleshooting section in SKILL.md for common issues and solutions.

## See Also

- [IAMGuarded Charts Building Guide](https://chainguard.engineering/docs/container-images/guides/delivery-guide-to-charts/) - Full documentation
- `/iamguarded-chart` skill - For creating chart packages
- enterprise-packages repository
- iamguarded-tools repository
