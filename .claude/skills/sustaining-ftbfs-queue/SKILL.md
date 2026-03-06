---
name: sustaining-ftbfs-queue
description: Review and merge FTBFS (Fails To Build From Source) PRs that are epoch-only bumps with all required CI checks passing. Use to clear the FTBFS queue at https://github.com/orgs/chainguard-dev/projects/60/views/43
---

# sustaining-ftbfs-queue

Work through the FTBFS queue, find all PRs that are safe to approve and merge
without manual intervention (epoch-only bumps with clean CI), present them to
the human for review, then approve and enable auto-merge on confirmation.

## Step 1: Fetch all open FTBFS PRs

The FTBFS queue is tracked at https://github.com/orgs/chainguard-dev/projects/60/views/43.
Fetch all open PRs in the stereo repo with the FTBFS bot title pattern:

```bash
gh pr list --repo chainguard-dev/stereo --state open \
  --search "failing to build from source" \
  --limit 200 --json number,headRefName \
  > /tmp/ftbfs_prs.json
python3 -c "import json; prs=json.load(open('/tmp/ftbfs_prs.json')); print(len(prs), 'open FTBFS PRs')"
```

## Step 2: Filter to epoch-only diffs

For each PR, check whether the diff touches exactly one file and changes only
the `epoch:` line. Discard any PR that modifies logic, adds dependencies,
changes pipeline steps, or touches more than one file.

```bash
python3 << 'EOF'
import subprocess, json

def run(cmd):
    return subprocess.run(cmd, shell=True, capture_output=True, text=True).stdout.strip()

prs = json.load(open('/tmp/ftbfs_prs.json'))
epoch_only = []

for pr in prs:
    num = pr['number']
    files = [f for f in run(f"gh pr diff {num} --name-only 2>/dev/null").splitlines() if f.strip()]
    if len(files) != 1:
        continue
    diff_lines = [l for l in run(f"gh pr diff {num} 2>/dev/null | grep '^[+-]' | grep -v '^---\\|^+++'").splitlines() if l.strip()]
    if diff_lines and all('epoch' in l for l in diff_lines):
        epoch_only.append({'number': num, 'file': files[0]})

json.dump(epoch_only, open('/tmp/epoch_only_prs.json', 'w'))
print(f"Epoch-only PRs: {len(epoch_only)}")
EOF
```

## Step 3: Check CI — elastic-build passing, no failures

For each epoch-only PR, verify:
- `elastic-build` check conclusion is `success`
- No check has conclusion `failure`

```bash
python3 << 'EOF'
import subprocess, json

def run(cmd):
    return subprocess.run(cmd, shell=True, capture_output=True, text=True).stdout.strip()

prs = json.load(open('/tmp/epoch_only_prs.json'))
candidates = []

for pr in prs:
    num = pr['number']
    sha = run(f"gh pr view {num} --json headRefOid --jq '.headRefOid' 2>/dev/null")
    if not sha:
        continue
    checks = run(f"gh api 'repos/chainguard-dev/stereo/commits/{sha}/check-runs?per_page=50' --jq '.check_runs[] | {{name, conclusion}}' 2>/dev/null")
    has_elastic_pass = False
    has_failure = False
    for line in checks.splitlines():
        try:
            c = json.loads(line)
            if c['name'] == 'elastic-build' and c['conclusion'] == 'success':
                has_elastic_pass = True
            if c['conclusion'] == 'failure':
                has_failure = True
        except:
            pass
    if has_elastic_pass and not has_failure:
        candidates.append(pr)

json.dump(candidates, open('/tmp/ready_candidates.json', 'w'))
print(f"Ready candidates: {len(candidates)}")
EOF
```

## Step 4: Print the candidate table for human review

Group by repo area (os, enterprise-packages, extra-packages) and print a
markdown table with PR number, package name, and clickable URL. **Stop here
and wait for the human to review and confirm before proceeding.**

```bash
python3 << 'EOF'
import json

candidates = json.load(open('/tmp/ready_candidates.json'))
groups = {}
for c in candidates:
    f = c['file']
    if f.startswith('os/'):
        g = 'os'
    elif f.startswith('enterprise-packages/'):
        g = 'enterprise-packages'
    else:
        g = 'extra-packages'
    groups.setdefault(g, []).append(c)

base = 'https://github.com/chainguard-dev/stereo/pull'
for g in ['os', 'enterprise-packages', 'extra-packages']:
    if g not in groups:
        continue
    items = sorted(groups[g], key=lambda x: x['number'])
    print(f"\n### {g} ({len(items)})")
    print("| PR | Package | URL |")
    print("|----|---------|-----|")
    for c in items:
        pkg = c['file'].split('/')[-1].replace('.yaml', '')
        print(f"| #{c['number']} | {pkg} | {base}/{c['number']} |")

print(f"\nTotal: {len(candidates)} PRs ready to approve and merge.")
EOF
```

Present this table to the human and ask: **"Shall I approve and enable
auto-merge on all N PRs above?"**

## Step 5: Approve and enable auto-merge (only after human confirms)

After the human confirms, approve each PR with a brief explanation and enable
squash auto-merge:

```bash
python3 << 'EOF'
import subprocess, json

def run(cmd):
    return subprocess.run(cmd, shell=True, capture_output=True, text=True)

candidates = json.load(open('/tmp/ready_candidates.json'))
merged_immediately = []
auto_merge_queued = []
failed = []

for pr in candidates:
    num = pr['number']
    # Approve
    run(f'gh pr review {num} --approve --body "Epoch-only bump; elastic-build passes, all required checks green."')
    # Enable auto-merge; if already in clean state it will merge immediately
    result = run(f'gh pr merge {num} --auto --squash')
    if 'clean status' in result.stderr:
        # Already mergeable — merge directly
        run(f'gh pr merge {num} --squash')
        merged_immediately.append(num)
    elif result.returncode == 0:
        auto_merge_queued.append(num)
    else:
        failed.append((num, result.stderr.strip()))

print(f"Merged immediately: {len(merged_immediately)}: {merged_immediately}")
print(f"Auto-merge queued:  {len(auto_merge_queued)}: {auto_merge_queued}")
if failed:
    print(f"Failed ({len(failed)}):")
    for num, err in failed:
        print(f"  #{num}: {err}")
EOF
```

## Notes

- **Only epoch-only diffs are considered safe for this workflow.** Any PR that
  changes build logic, dependencies, pipeline steps, or test configuration
  requires manual review and must not be included.
- The `elastic-build` check is the primary required check for stereo PRs. A PR
  that passes elastic-build but has a failing non-required check (e.g.,
  `ci-cve-scan-db`, `ci-sbom-validity`) is still eligible — those failures
  indicate pre-existing issues unrelated to the epoch bump.
- FTBFS PRs are auto-generated by the FTBFS bot when a package fails
  consecutive production builds. An epoch bump forces a clean rebuild against
  the current dependency set, which often resolves transient failures caused by
  upstream dependency changes (e.g., soname bumps, API changes).
- After merging, the production build system will pick up the new epoch and
  confirm the fix. If the build fails again in production, the FTBFS bot will
  open a new PR.
