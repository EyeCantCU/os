---
name: pr-review
description: Review a GitHub PR with full context (enterprise-packages)
---

# pr-review

You are conducting a PR review for the enterprise-packages repository (chainguard-dev/enterprise-packages).

### Notes:
- The shell script pr-review-functions.sh is in the Claude skill directory in `scripts`, e.g., `./.claude/skills/pr-review/./.claude/skills/pr-review/scripts/pr-review-functions.sh`


## Setup Process

To set up the PR review environment, follow these steps in order:

1. **Check Requirements**: First, verify all required tools are available:
   ```bash
   ./.claude/skills/pr-review/scripts/pr-review-functions.sh check_requirements
   ```

2. **Create Temporary Directory**: Create a temp directory for the review:
   ```bash
   TEMP_DIR=$(./.claude/skills/pr-review/scripts/pr-review-functions.sh setup_temp_dir)
   echo "Created temp directory: $TEMP_DIR"
   ```

3. **Clone Repository**: Clone enterprise-packages into the temp directory:
   ```bash
   ./.claude/skills/pr-review/scripts/pr-review-functions.sh clone_repo "$TEMP_DIR"
   ```

4. **Checkout PR**: Checkout the specific PR (where PR_NUMBER is the PR to review):
   ```bash
   ./.claude/skills/pr-review/scripts/pr-review-functions.sh checkout_pr PR_NUMBER "$TEMP_DIR"
   ```

5. **Navigate to Repository**: Change to the temp directory:
   ```bash
   cd "$TEMP_DIR"
   ```

6. **Get PR Information** (optional but recommended): Fetch PR details:
   ```bash
   ./.claude/skills/pr-review/scripts/pr-review-functions.sh get_pr_info PR_NUMBER
   ```

## Review Process

Once the environment is set up and you're in the repository directory, analyze the changes in this PR and focus on identifying issues related to:

- **Correctness**: Are there any issues in the code?
- **Code Quality**: Is the code readable, maintainable, and following best practices?
- **Testing**: Are the package tests adequate? The tests should prove the functionality of the application under test.
- **Error Handling**: Are errors handled properly? For example, a failing test typically shouldn't be ignored with `|| true`.
- **Breaking Changes**: Does the change introduce any breaking changes to an existing package?

### Repository-Specific Guidelines

Check for the following files in the repository. If present, read them and use them to provide feedback:
- CLAUDE.md
- STYLEGUIDE.md
- CODING_STYLE.md
- STYLE.md
- CONTRIBUTING.md

Changes that do not follow these guides should be included in your feedback.

### Review the Changes

Use git commands to understand what changed:
```bash
# See the diff from main branch
git diff main...HEAD

# List changed files
git diff --name-only main...HEAD

# See commit history
git log main..HEAD --oneline
```

Read the relevant files that were changed and analyze them thoroughly.

## Be Constructive

- Be respectful and assume good intent
- Explain *why* something should change, not just *what*
- Suggest concrete improvements
- Acknowledge good work
- Focus on the code, not the person

## Output Format

When providing file-specific feedback, include references to the file path and line numbers (e.g., `src/server.ts:42`).

Structure your review as follows:

**Summary**
[What does this PR add or change? What is the overall assessment?]

**Critical Issues**
[Issues that must be fixed before merging. If none, state "None identified."]

**Suggestions**
[Nice-to-have improvements that won't prevent merging. If none, state "None."]

**Positive Notes**
[Acknowledge good practices, clever solutions, or well-written code]

## Cleanup

After completing the review, ask the user to confirm and then cleanup the temporary directory. Allow the user to cleanup the temporary directory with the keyword "cleanup." Use this command to clean up the directory:
```bash
./.claude/skills/pr-review/scripts/pr-review-functions.sh cleanup_temp_dir "$TEMP_DIR"
```

## Important Notes

- This skill only works with the enterprise-packages repository
- You must have access to the repository via gh CLI authentication
- The PR must exist and be accessible
- All bash commands should be run using the Bash tool
- Store the TEMP_DIR value in a variable after creation to use in subsequent commands
- Always clean up the temp directory when done, even if the review encounters errors
