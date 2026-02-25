# How To Patch CVEs

The process of patching Stereo APK packages for reported CVEs.

## Finding the Advisories

To find the Advisory for a CVE:

```sh
cg adv list --vuln-id ${VULN_ID} --origin ${PACKAGE_NAME}
```

where:

- `${VULN_ID}` - the CVE ID (such as `CVE-2026-1225`) or GitHub ID (such as
  `GHSA-qqpg-mvqg-649v`)
- `${PACKGE_NAME}` - our package name (such as `seata`)

For example:

<!-- markdownlint-disable MD013 -->
```sh
$ cg adv list --vuln-id CVE-2026-1225 --origin zookeeper-3.8
---------------------|---------------|------|---------|------------------|--------|--------|-----------------------
| ID                 | NAME          | TYPE | ARCH    | VULN ID          | EVENTS | STATUS | UPDATED              |
|--------------------|---------------|------|---------|------------------|--------|--------|----------------------|
| CGA-rqjc-22xv-728x | zookeeper-3.8 | apk  | x86_64  | CVE-2026-1225 +2 | 3      | fixed  | 2026-02-09 20:17 UTC |
| CGA-6w7p-8vr3-qj49 | zookeeper-3.8 | apk  | x86_64  | CVE-2026-1225 +2 | 3      | fixed  | 2026-02-09 20:17 UTC |
| CGA-2gr2-c9hp-557j | zookeeper-3.8 | apk  | aarch64 | CVE-2026-1225 +2 | 3      | fixed  | 2026-02-09 20:17 UTC |
| CGA-2mjq-9rxg-xm8w | zookeeper-3.8 | apk  | x86_64  | CVE-2026-1225 +2 | 3      | fixed  | 2026-02-09 20:17 UTC |
| CGA-qcgp-x9fr-v4rh | zookeeper-3.8 | apk  | aarch64 | CVE-2026-1225 +2 | 3      | fixed  | 2026-02-09 20:17 UTC |
| CGA-hcmw-f3xw-xqj4 | zookeeper-3.8 | apk  | aarch64 | CVE-2026-1225 +2 | 3      | fixed  | 2026-02-09 20:17 UTC |
---------------------|---------------|------|---------|------------------|--------|--------|-----------------------

Fixed: 6  Total shown: 6
```
<!-- markdownlint-enable MD013 -->

Shows five separate advisories for CVE-2026-1225 attached to `zookeeper-3.8`.

These Advisory IDs are what you need to create new *events* to indicate a CVE
has been fixed or is pending an upstream fix, etc.

### There Are No Advisories

If there are no advisories for the CVE:

- Why didn't automation detect this?
- Has a version bump or other change already merged to `main` addressed it?

You may need to manually create a detection event, but this should be rare.

## NACKing a CVE

If the package isn't actually affected by the detected CVE, NACK it.

To NACK a CVE for a given package, we don't add in any patches or increment the
package version. Instead, we just need to update the advisory data to say that
this package is not affected by the vulnerability. As we do this, we also need
to provide an accurate
["justification"](https://github.com/chainguard-dev/vex/blob/main/pkg/vex/justification.go#L12-L49)
value as to why our package isn't affected.

**TODO:** Update for current reality.

Recording the NACK is done with the `wolfictl adv create` command (or `wolfictl
adv update` if an advisory already exists for this package and CVE).

For example:

**TODO:** Update for current reality.

```sh
$ wolfictl adv create Auto-detected distro: Wolfi

Package: zlib Vulnerability: CVE-2023-77777 Status: not_affected Justification:
vulnerable_code_not_present
```

This will notify vulnerability scanners that consume our secdb that this
vulnerabitiliy doesn't apply to any of the versions of this package that we've
published.

## Patch Steps

For the ease of explanation, we'll assume we're addressing a single reported
vulnerability for a single package.

### Find It

1. Determine the vulnerability's CVE ID. (e.g. "CVE-2018-25032")
2. Locate the best patch for the affected software.
   a. If possible, find a patch in the "upstream" repo for this software,
      linked from the vulnerability report itself (e.g. from
      `https://nvd.nist.gov/vuln/detail/<CVE-ID>`).
   b. If an upstream patch is not available, look to see how other Linux
      distros have handled patching for this CVE. (Tip: Alpine and Fedora are
      both pretty good at patching!)
   c. It's possible that patching is not possible or not appropriate for the
      situation. If the vulnerability has never been exploitable in this
      package, you can "NACK" (negatively acknowledge) this CVE for this
      package. See [NACKing a CVE](#nacking-a-cve) for instructions, and skip
      all remaining steps.

### Fix It

If you can address the vuln by bumping dependencies:

1. In your local clone of the repo, edit the `{package}.yaml` file.
2. Increment the `epoch` value. (e.g. change from `2` to `3`)
3. Bump the vulnerable dependencies:
   <https://eng.inky.wtf/docs/on-boarding/sustaining/work-queues/cve_queue/remediation-patterns-guides/advanced/dependency-bumps/>

If an upstream patch is available for the version you're fixing, cherry-pick it:

1. In your local clone of the repo, edit the `{package}.yaml` file.
2. Increment the `epoch` value. (e.g. change from `2` to `3`)
3. Add a `cherry-picks` entry if one isn't already there, or add a line to the
   list of commits:

   ```yaml
   cherry-picks: |
     {branch}/{hash}: [${VULN_ID}] Description of the problem or change.
   ```

   where:

   - `{branch}` - branch name (like or `3.14`, etc.)
   - `{hash}` - commit hash
   - Add the `${VULN_ID}` and a description of the vuln or the change (grabbing
     the first line of the commit message is a good choice.)

If you need to add a patch that can't be pulled in via `cherry-picks`:

1. In your local clone of this repo, create a top-level directory using the name
   of the affected package, if such a directory doesn't already exist.
2. Download the patch into this package directory. The patch's file name should
   be in the form of `<CVE-ID>.patch` (e.g. `CVE-2018-25032.patch`).
3. Edit the new `.patch` file to indicate where it came from (the URL to the
   upstream commit is a good choice) and why you needed to create the `.patch`
   instead of using a `cherry-pick`.
4. In the Melange YAML file for this package (back in the root of the repo),
   make the following updates:
   a. Increment the "epoch" value. (e.g. change from `2` to `3`)
   b. For the new patch, add a `patch` pipeline item in the `pipeline` YAML
      section. For example, if we downloaded a patch called
      `CVE-2018-25032.patch` to our package directory above, we'd add an item
      to `pipeline` that looks like this:

      ```yaml
      - uses: patch
        with:
          patches: |
            CVE-2018-25032.patch
      ```

    For more information on how `patch` works, see [its
    definition](https://github.com/chainguard-dev/melange/blob/main/pkg/build/pipelines/patch.yaml).

If there *isn't* a patch available for this issue, and there *isn't* a newer
version of the package that fixes it, document that it's `pending-upstream-fix`:

1. Create a `pending-upstream-fix` event:

   ```sh
   cg adv event create \
       --advisory-id ${ADVISORY_ID} \
       --type pending-upstream-fix \
       --note "${IMPORTANT_NOTE}"
   ```

   **Note:** You have to do this for every `${ADVISORY_ID` you found in the
   *Finding the Advisories* section of this doc.

   **Note:** The `${IMPORTANT_NOTE}` is required for your `pending-upstream-fix`
   event to be approved; it should include:

   - the state of this issue (not fixed at all, fixed in `main` but not the
     release, etc.)
   - any other relevant info (expected fix date if there is one, reasons why it's
     not getting fixed, etc.)
2. Skip the *Verify It* and *Document It* sections.

### Verify It

Verify that our update package will build successfully by running Melange. To
do this, run (in a container if you're not already on Linux):

```sh
make debug/${PACKAGE_NAME} test-debug/${PACKAGE_NAME}
```

For some packages (like Go applications), you can scan the resulting `.apk`
files to see if you change has addressed the issue:

```sh
cg scan apk packages/${ARCH}/${PACKAGE_NAME}
```

If everything looks good, you can open a PR.

Once the PR is merged to `main`, automation should create a `fixed` event.
