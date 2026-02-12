# IAMGuarded Chart Skill

Claude skill for automating the creation of IAMGuarded chart APK packages.

## Overview

This skill automates chart package creation (Part I of the IAMGuarded workflow):
- Creates chart APK packages (chart-iamguarded-{name})
- Extracts metadata from iamguarded-charts repository
- Generates patch files to remove Bitnami references
- Builds and tests the chart package

For creating IAMGuarded compat subpackages (Part II), use the separate `/iamguarded-package` skill.

# Quickstart

Run the skill with the shell script (replace CHART with the package you want to generate generate iamguarded packages for)

```bash
./.claude/skills/iamguarded-chart/run-skill.sh CHART
```

## Files

### SKILL.md
The Claude skill definition that provides step-by-step instructions for the LLM. The workflow is organized into 6 phases:

1. **Gather Requirements** - Collect chart name
2. **Extract Chart Metadata** - Clone repo and get version/dependencies
3. **Create Chart Package** - Generate chart package YAML
4. **Generate Patch File** - Remove Bitnami references from NOTES.txt
5. **Build and Test Chart** - Compile and validate the chart
6. **Cleanup and Summary** - Remove temp files and report results

### server.py
Python-based MCP server that provides tools for chart creation:

**MCP Tools:**
- `checkout_iamguarded_charts` - Clone iamguarded-charts to temp directory
- `cleanup` - Remove temporary directory
- `get_latest_version` - Get latest version tag for a chart
- `get_latest_hash` - Get commit hash for a specific version
- `get_chart_dependencies` - Extract chart dependencies from Chart.yaml
- `get_chart_image_dependencies` - Extract image dependencies from Chart.yaml annotations
- `create_chart` - Generate chart package YAML from template
- `generate_chart` - Wrapper that combines multiple operations

**Features:**
- Uses pure Python with pyyaml for YAML processing
- Maintains state to avoid re-cloning repositories
- Proper error handling and validation
- Clear error messages
- Follows the pattern from iamguarded-package skill

### chart-template.tmpl
YAML template used by `create_chart` to generate chart package files. Contains placeholders that are replaced with actual values using pyyaml.

### run-skill.sh
Wrapper script that manages the MCP server lifecycle:
- Installs Python dependencies with uv
- Registers the MCP server in local scope
- Launches Claude with the skill
- Cleans up MCP registration on exit

## Usage

1. Ensure prerequisites are met:
   - In enterprise-packages repository root
   - GitHub CLI authenticated (`gh auth login`)
   - Access to iamguarded-charts repo

2. Invoke the skill:
   ```
   /iamguarded-chart
   ```

3. Provide requested information:
   - Chart name (e.g., "mysql", "postgresql")

4. Follow the prompts and review generated files

5. After completion, use `/iamguarded-package` skill for compat subpackages

## Testing

The MCP server is automatically managed by the run-skill.sh wrapper script. To test:

1. **Test the full skill:**
   ```bash
   ./run-skill.sh
   ```
   This will register the MCP server and launch Claude with the skill.

2. **Test MCP tools manually:**
   Within a Claude session with the skill loaded, you can test individual tools:
   - `checkout_iamguarded_charts` - Test cloning the repo
   - `get_latest_version` - Test getting version for a chart (requires repo_dir and chart_name)
   - `cleanup` - Test removing temporary directory

## Troubleshooting

See the Troubleshooting section in SKILL.md for common issues and solutions.

## See Also

- [IAMGuarded Charts Building Guide](https://www.notion.so/chainguard/Building-Chainguard-Helm-Charts) - Full documentation
- `/iamguarded-package` skill - For creating IAMGuarded compat subpackages
- enterprise-packages repository
- iamguarded-charts repository
