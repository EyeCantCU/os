#!/usr/bin/env python3
"""
IAMGuarded Chart MCP Server

Provides tools for automating IAMGuarded chart package creation.
"""

import os
import subprocess
import tempfile
import shutil
import json
from pathlib import Path
from typing import Optional, Dict, Any, List
import yaml

from mcp.server import Server
from mcp.types import Tool, TextContent

# Global state for managing temporary directories
_temp_repos: Dict[str, str] = {}

app = Server("iamguarded-chart")


def validate_working_directory() -> Optional[str]:
    """Validate that we're in the enterprise-packages repository root."""
    if not Path("enterprise-packages").exists() and not Path(".git").exists():
        return "ERROR: Must be run from enterprise-packages repository root"

    # Check if this is actually a git repo
    returncode, stdout, stderr = run_command([
        "git", "rev-parse", "--git-dir"
    ])
    if returncode != 0:
        return f"ERROR: {stderr}"
    return None


def run_command(cmd: list[str], cwd: Optional[str] = None) -> tuple[int, str, str]:
    """Run a command and return exit code, stdout, stderr."""
    try:
        result = subprocess.run(
            cmd,
            cwd=cwd,
            capture_output=True,
            text=True
        )
        return result.returncode, result.stdout, result.stderr
    except Exception as e:
        return 1, "", str(e)


@app.list_tools()
async def list_tools() -> list[Tool]:
    """List available tools."""
    return [
        Tool(
            name="checkout_iamguarded_charts",
            description="Clone the iamguarded-charts repository to a temporary directory. Returns the path to the cloned repo. Maintains state to avoid re-cloning.",
            inputSchema={
                "type": "object",
                "properties": {},
                "required": []
            }
        ),
        Tool(
            name="cleanup",
            description="Remove a temporary directory created by checkout_iamguarded_charts.",
            inputSchema={
                "type": "object",
                "properties": {
                    "temp_dir": {
                        "type": "string",
                        "description": "Path to the temporary directory to remove"
                    }
                },
                "required": ["temp_dir"]
            }
        ),
        Tool(
            name="get_latest_version",
            description="Get the latest version tag for a chart from the iamguarded-charts repository.",
            inputSchema={
                "type": "object",
                "properties": {
                    "repo_dir": {
                        "type": "string",
                        "description": "Path to the cloned iamguarded-charts repository"
                    },
                    "chart_name": {
                        "type": "string",
                        "description": "Chart name (e.g., 'mysql', 'postgresql')"
                    }
                },
                "required": ["repo_dir", "chart_name"]
            }
        ),
        Tool(
            name="get_latest_hash",
            description="Get the commit hash for a specific chart version.",
            inputSchema={
                "type": "object",
                "properties": {
                    "repo_dir": {
                        "type": "string",
                        "description": "Path to the cloned iamguarded-charts repository"
                    },
                    "chart_name": {
                        "type": "string",
                        "description": "Chart name (e.g., 'mysql', 'postgresql')"
                    },
                    "version": {
                        "type": "string",
                        "description": "Chart version (e.g., '1.2.3')"
                    }
                },
                "required": ["repo_dir", "chart_name", "version"]
            }
        ),
        Tool(
            name="get_chart_dependencies",
            description="Extract chart dependencies from Chart.yaml. Returns dependencies as newline-separated string (or empty string if none).",
            inputSchema={
                "type": "object",
                "properties": {
                    "repo_dir": {
                        "type": "string",
                        "description": "Path to the cloned iamguarded-charts repository"
                    },
                    "chart_name": {
                        "type": "string",
                        "description": "Chart name (e.g., 'mysql', 'postgresql')"
                    },
                    "version": {
                        "type": "string",
                        "description": "Chart version (not currently used but kept for consistency)"
                    }
                },
                "required": ["repo_dir", "chart_name"]
            }
        ),
        Tool(
            name="get_chart_image_dependencies",
            description="Extract image dependencies from Chart.yaml annotations. Returns image names as newline-separated string.",
            inputSchema={
                "type": "object",
                "properties": {
                    "repo_dir": {
                        "type": "string",
                        "description": "Path to the cloned iamguarded-charts repository"
                    },
                    "chart_name": {
                        "type": "string",
                        "description": "Chart name (e.g., 'mysql', 'postgresql')"
                    },
                    "version": {
                        "type": "string",
                        "description": "Chart version (not currently used but kept for consistency)"
                    }
                },
                "required": ["repo_dir", "chart_name"]
            }
        ),
        Tool(
            name="create_chart",
            description="Generate a chart package YAML file from template by substituting values.",
            inputSchema={
                "type": "object",
                "properties": {
                    "chart_name": {
                        "type": "string",
                        "description": "Chart name (e.g., 'mysql', 'postgresql')"
                    },
                    "version": {
                        "type": "string",
                        "description": "Chart version (e.g., '1.2.3')"
                    },
                    "dependencies": {
                        "type": "string",
                        "description": "Newline-separated list of chart dependencies (can be empty)"
                    },
                    "hash": {
                        "type": "string",
                        "description": "Git commit hash for the chart version"
                    },
                    "template_path": {
                        "type": "string",
                        "description": "Path to the chart template file"
                    },
                    "output_path": {
                        "type": "string",
                        "description": "Path where the generated chart package YAML should be written"
                    }
                },
                "required": ["chart_name", "version", "dependencies", "hash", "template_path", "output_path"]
            }
        ),
        Tool(
            name="generate_chart",
            description="Wrapper function that combines get_latest_version, get_latest_hash, get_chart_dependencies, and create_chart to generate a complete chart package YAML in one step.",
            inputSchema={
                "type": "object",
                "properties": {
                    "repo_dir": {
                        "type": "string",
                        "description": "Path to the cloned iamguarded-charts repository"
                    },
                    "chart_name": {
                        "type": "string",
                        "description": "Chart name (e.g., 'mysql', 'postgresql')"
                    },
                    "template_path": {
                        "type": "string",
                        "description": "Path to the chart template file"
                    },
                    "output_path": {
                        "type": "string",
                        "description": "Path where the generated chart package YAML should be written"
                    }
                },
                "required": ["repo_dir", "chart_name", "template_path", "output_path"]
            }
        ),
    ]


@app.call_tool()
async def call_tool(name: str, arguments: Any) -> list[TextContent]:
    """Handle tool calls."""

    # Validate working directory for all tools
    error = validate_working_directory()
    if error:
        return [TextContent(type="text", text=error)]

    if name == "checkout_iamguarded_charts":
        return await checkout_iamguarded_charts()
    elif name == "cleanup":
        return await cleanup(arguments["temp_dir"])
    elif name == "get_latest_version":
        return await get_latest_version(arguments["repo_dir"], arguments["chart_name"])
    elif name == "get_latest_hash":
        return await get_latest_hash(arguments["repo_dir"], arguments["chart_name"], arguments["version"])
    elif name == "get_chart_dependencies":
        return await get_chart_dependencies(arguments["repo_dir"], arguments["chart_name"], arguments.get("version", ""))
    elif name == "get_chart_image_dependencies":
        return await get_chart_image_dependencies(arguments["repo_dir"], arguments["chart_name"], arguments.get("version", ""))
    elif name == "create_chart":
        return await create_chart(
            arguments["chart_name"],
            arguments["version"],
            arguments["dependencies"],
            arguments["hash"],
            arguments["template_path"],
            arguments["output_path"]
        )
    elif name == "generate_chart":
        return await generate_chart(
            arguments["repo_dir"],
            arguments["chart_name"],
            arguments["template_path"],
            arguments["output_path"]
        )
    else:
        return [TextContent(type="text", text=f"ERROR: Unknown tool: {name}")]


async def checkout_iamguarded_charts() -> list[TextContent]:
    """Clone the iamguarded-charts repository to a temporary directory."""

    repo_name = "iamguarded-charts"

    # Check if already cloned
    if repo_name in _temp_repos:
        repo_path = _temp_repos[repo_name]
        if Path(repo_path).exists():
            return [TextContent(
                type="text",
                text=f"Repository already cloned at: {repo_path}"
            )]

    # Create temp directory
    temp_base = tempfile.mkdtemp(prefix="iamguarded-chart-")
    repo_path = Path(temp_base) / repo_name

    # Clone the repository
    repo_url = f"https://github.com/chainguard-dev/{repo_name}.git"
    returncode, stdout, stderr = run_command([
        "git", "clone", "-q", "--depth", "50", repo_url, str(repo_path)
    ])

    if returncode != 0:
        shutil.rmtree(temp_base, ignore_errors=True)
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to clone {repo_name} repository\n{stderr}"
        )]

    # Store the path
    _temp_repos[repo_name] = str(repo_path)

    return [TextContent(
        type="text",
        text=str(repo_path)
    )]


async def cleanup(temp_dir: str) -> list[TextContent]:
    """Remove a temporary directory."""

    if not temp_dir:
        return [TextContent(type="text", text="ERROR: temp_dir parameter is required")]

    temp_path = Path(temp_dir)

    if not temp_path.exists():
        return [TextContent(
            type="text",
            text=f"Warning: Directory does not exist: {temp_dir}"
        )]

    try:
        shutil.rmtree(temp_dir)

        # Remove from tracked repos
        for repo_name, path in list(_temp_repos.items()):
            if path == temp_dir or str(temp_path) in path:
                del _temp_repos[repo_name]

        return [TextContent(
            type="text",
            text=f"Cleaned up temporary directory: {temp_dir}"
        )]
    except Exception as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to remove directory {temp_dir}: {str(e)}"
        )]


async def get_latest_version(repo_dir: str, chart_name: str) -> list[TextContent]:
    """Get the latest version tag for a chart."""

    if not repo_dir or not chart_name:
        return [TextContent(type="text", text="ERROR: repo_dir and chart_name parameters are required")]

    repo_path = Path(repo_dir)
    if not repo_path.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Directory does not exist: {repo_dir}"
        )]

    # Fetch more tags to ensure we get recent versions
    returncode, stdout, stderr = run_command([
        "git", "fetch", "-q", "--tags", "--depth=100"
    ], cwd=repo_dir)

    if returncode != 0:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to fetch tags from repository\n{stderr}"
        )]

    # Get latest version tag for this chart
    returncode, stdout, stderr = run_command([
        "git", "tag", "-l", f"{chart_name}/*", "--sort=v:refname"
    ], cwd=repo_dir)

    if returncode != 0:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to get tags\n{stderr}"
        )]

    tags = stdout.strip().split('\n')
    if not tags or not tags[0]:
        return [TextContent(
            type="text",
            text=f"ERROR: No tags found matching pattern: {chart_name}/*"
        )]

    # Get the last tag and remove the prefix
    latest_tag = tags[-1]
    version = latest_tag.replace(f"{chart_name}/", "", 1)

    return [TextContent(type="text", text=version)]


async def get_latest_hash(repo_dir: str, chart_name: str, version: str) -> list[TextContent]:
    """Get the commit hash for a specific chart version."""

    if not repo_dir or not chart_name or not version:
        return [TextContent(type="text", text="ERROR: repo_dir, chart_name, and version parameters are required")]

    repo_path = Path(repo_dir)
    if not repo_path.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Directory does not exist: {repo_dir}"
        )]

    tag = f"{chart_name}/{version}"

    returncode, stdout, stderr = run_command([
        "git", "rev-parse", tag
    ], cwd=repo_dir)

    if returncode != 0:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to get commit hash for tag: {tag}\n{stderr}"
        )]

    commit_hash = stdout.strip()
    return [TextContent(type="text", text=commit_hash)]


async def get_chart_dependencies(repo_dir: str, chart_name: str, version: str = "") -> list[TextContent]:
    """Extract chart dependencies from Chart.yaml."""

    if not repo_dir or not chart_name:
        return [TextContent(type="text", text="ERROR: repo_dir and chart_name parameters are required")]

    chart_path = Path(repo_dir) / "bitnami" / chart_name

    if not chart_path.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Chart path does not exist: {chart_path}"
        )]

    chart_yaml = chart_path / "Chart.yaml"
    if not chart_yaml.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Chart.yaml not found in: {chart_path}"
        )]

    try:
        with open(chart_yaml, 'r') as f:
            data = yaml.safe_load(f)

        # Get dependencies
        dependencies = data.get('dependencies', [])
        dep_names = []
        for dep in dependencies:
            if 'name' in dep:
                dep_names.append(dep['name'])

        # Return as newline-separated string
        return [TextContent(type="text", text='\n'.join(dep_names))]

    except Exception as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to parse Chart.yaml: {str(e)}"
        )]


async def get_chart_image_dependencies(repo_dir: str, chart_name: str, version: str = "") -> list[TextContent]:
    """Extract image dependencies from Chart.yaml annotations."""

    if not repo_dir or not chart_name:
        return [TextContent(type="text", text="ERROR: repo_dir and chart_name parameters are required")]

    chart_path = Path(repo_dir) / "bitnami" / chart_name

    if not chart_path.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Chart path does not exist: {chart_path}"
        )]

    chart_yaml = chart_path / "Chart.yaml"
    if not chart_yaml.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Chart.yaml not found in: {chart_path}"
        )]

    try:
        with open(chart_yaml, 'r') as f:
            data = yaml.safe_load(f)

        # Get annotations.images
        annotations = data.get('annotations', {})
        images_yaml = annotations.get('images', '')

        if not images_yaml:
            return [TextContent(type="text", text="")]

        # Parse the nested YAML in annotations.images
        images_data = yaml.safe_load(images_yaml)
        if not images_data:
            return [TextContent(type="text", text="")]

        # Extract image names
        image_names = []
        for img in images_data:
            if 'name' in img:
                image_names.append(img['name'])

        # Return as newline-separated string
        return [TextContent(type="text", text='\n'.join(image_names))]

    except Exception as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to parse Chart.yaml annotations: {str(e)}"
        )]


async def create_chart(
    chart_name: str,
    version: str,
    dependencies: str,
    hash: str,
    template_path: str,
    output_path: str
) -> list[TextContent]:
    """Generate a chart package YAML file from template."""

    if not chart_name or not version or not hash or not template_path or not output_path:
        return [TextContent(type="text", text="ERROR: chart_name, version, hash, template_path, and output_path are required")]

    template_file = Path(template_path)
    if not template_file.exists():
        return [TextContent(
            type="text",
            text=f"ERROR: Template file does not exist: {template_path}"
        )]

    try:
        # Load template
        with open(template_path, 'r') as f:
            template_data = yaml.safe_load(f)

        # Prepare values
        package_name = f"chart-iamguarded-{chart_name}"
        package_description = f"{chart_name} iamguarded chart"
        git_tag = f"{chart_name}/{version}"
        expected_commit = hash
        chart_path = f"iamguarded/{chart_name}"
        strip_prefix = f"{chart_name}/"
        validate_chart_name = f"/{chart_name}"

        # Process dependencies: add chart-iamguarded- prefix to each non-empty line
        processed_deps = []
        if dependencies:
            for dep in dependencies.split('\n'):
                dep = dep.strip()
                if dep:
                    processed_deps.append(f"chart-iamguarded-{dep}")

        # Update template data
        template_data['package']['name'] = package_name
        template_data['package']['version'] = version
        template_data['package']['description'] = package_description
        template_data['environment']['contents']['packages'] = processed_deps

        # Update pipeline steps
        for step in template_data['pipeline']:
            if step.get('uses') == 'git-checkout':
                step['with']['tag'] = git_tag
                step['with']['expected-commit'] = expected_commit
            elif step.get('uses') == 'iamguarded/prepare':
                step['with']['chart-path'] = chart_path
            elif step.get('uses') == 'charts/package':
                step['with']['chart-path'] = chart_path

        # Update update section
        template_data['update']['github']['strip-prefix'] = strip_prefix
        template_data['update']['github']['tag-filter-prefix'] = strip_prefix

        # Update test pipeline
        for step in template_data['test']['pipeline']:
            if step.get('uses') == 'iamguarded/validate':
                step['with']['chart-name'] = validate_chart_name

        # Write output file
        with open(output_path, 'w') as f:
            yaml.dump(template_data, f, default_flow_style=False, sort_keys=False)

        return [TextContent(
            type="text",
            text=f"Created chart package file: {output_path}"
        )]

    except Exception as e:
        return [TextContent(
            type="text",
            text=f"ERROR: Failed to create chart package file: {str(e)}"
        )]


async def generate_chart(
    repo_dir: str,
    chart_name: str,
    template_path: str,
    output_path: str
) -> list[TextContent]:
    """Wrapper that combines multiple operations to generate a complete chart package."""

    if not repo_dir or not chart_name or not template_path or not output_path:
        return [TextContent(type="text", text="ERROR: All parameters are required")]

    # Get latest version
    version_result = await get_latest_version(repo_dir, chart_name)
    if version_result[0].text.startswith("ERROR"):
        return version_result
    version = version_result[0].text

    # Get commit hash
    hash_result = await get_latest_hash(repo_dir, chart_name, version)
    if hash_result[0].text.startswith("ERROR"):
        return hash_result
    hash = hash_result[0].text

    # Get dependencies
    deps_result = await get_chart_dependencies(repo_dir, chart_name, version)
    if deps_result[0].text.startswith("ERROR"):
        return deps_result
    dependencies = deps_result[0].text

    # Create chart
    return await create_chart(chart_name, version, dependencies, hash, template_path, output_path)


if __name__ == "__main__":
    import asyncio
    from mcp.server.stdio import stdio_server

    async def main():
        async with stdio_server() as (read_stream, write_stream):
            await app.run(
                read_stream,
                write_stream,
                app.create_initialization_options()
            )

    asyncio.run(main())
