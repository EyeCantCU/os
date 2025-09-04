# Trial

Welcome to stereo!
Thank you for your willingness to participate in this experiment.

## Workflow

During the trial, we won't actually be merging any package changes into stereo.
Because we'll sync all package defintions from wolfi, extras, and enterprise into stereo directly, merging package changes into stereo would cause conflicts.
Instead, we'll just be making changes, opening Pull Requests, observing the results of the CI, and closing them.

Your job is to evaluate the effectiveness of stereo and its CI to catch common failures that we would otherwise miss.

## Getting Started

We fully intend to automate all of this, but currently things are quite janky.

Before making any changes, you'll want to sync upstream sources into stereo:

```
./hack/sync.sh
```

And install the `stereo` cli:

```
go install ./cmd/stereo
```

### Building Packages

For the time being, we have a `Makefile` in the root directory that should feel fairly familiar to the old repositories.
All this does is multiplex across each subdirectory by bouncing through stereo to locate a given package and wire up additional flags.

```
# You do not have to cd into os/ first, yay.
make package/crane
```

We can absolutely do better than this -- feedback welcome on your favorite build system.

### Detecting Duplicate Packages

Then we'll make a small change, e.g. we can simulate adding a new enterprise package that already exists in wolfi:

```
cp os/crane.yaml enterprise-packages/
```

One of our goals is to prevent us from accidentally doing this.
Let's see if `stereo lint` catches that:

```
stereo lint
Error: conflict: "tensorrt-10.0-dev" in extra-packages/tensorrt-10.0.yaml and enterprise-packages/tensorrt-10.0.yaml
conflict: "tensorrt-10.0" in extra-packages/tensorrt-10.0.yaml and enterprise-packages/tensorrt-10.0.yaml
conflict: "crane" in enterprise-packages/crane.yaml and os/crane.yaml
conflict: "asciidoctor" in enterprise-packages/asciidoctor.yaml and os/asciidoctor.yaml
conflict: "crane-cov" in enterprise-packages/crane.yaml and os/crane.yaml
exit status 1
```

Indeed we see that we're trying to introduce a package that already exists.
This is a contrived example, but you can see that we already had some pre-existing problems with `asciidoctor` and `tensorrt-10.0`.
Perhaps we should fix that!

### Determining Blast Radius

Another goal of stereo is to avoid accidentally breaking extras and enterprise all the time.
To start, we'll look at what package builds are affected by a given change.

As an example, let's bump `curl` and see how large of a change that is:

```
make clean
(cd os && wolfictl bump curl) # We should probably implement `stereo bump` or something.
make package/curl
stereo impact | tail -n 20

2025/08/29 14:18:38 before: saw 7 errors
2025/08/29 14:18:55 after: saw 7 errors
2025/08/29 14:18:55 0 builds have new failures
2025/08/29 14:18:55 4579 melange builds affected by new packages
os/zot:
  - curl=8.15.0-r5
  + curl=8.15.0-r6
  - libcurl-openssl4=8.15.0-r5
  + libcurl-openssl4=8.15.0-r6
os/zoxide:
  - libcurl-openssl4=8.15.0-r5
  + libcurl-openssl4=8.15.0-r6
os/zsh:
  - libcurl-openssl4=8.15.0-r5
  + libcurl-openssl4=8.15.0-r6
os/zstd:
  - libcurl-openssl4=8.15.0-r5
  + libcurl-openssl4=8.15.0-r6
os/ztunnel-1.26:
  - libcurl-openssl4=8.15.0-r5
  + libcurl-openssl4=8.15.0-r6
os/ztunnel-1.27:
  - libcurl-openssl4=8.15.0-r5
  + libcurl-openssl4=8.15.0-r6
```

Wow!
4579 builds is a lot!

```
ls os extra-packages enterprise-packages | grep yaml | wc -l
    5718
```

That's almost all of them!
I had to `tail` for this to be a reasonable size.

What if we do the same thing for something less critical to our package builds, like `crane`?

```
make clean
(cd os && wolfictl bump crane)
make package/crane
stereo impact

2025/08/29 14:24:09 before: saw 7 errors
2025/08/29 14:24:25 after: saw 7 errors
enterprise-packages/k3s-1.31:
  - crane=0.20.6-r2
  + crane=0.20.6-r3
enterprise-packages/vunnel:
  - crane=0.20.6-r2
  + crane=0.20.6-r3
os/k3s:
  - crane=0.20.6-r2
  + crane=0.20.6-r3
os/k3s-1.32:
  - crane=0.20.6-r2
  + crane=0.20.6-r3
os/neuvector-db:
  - crane=0.20.6-r2
  + crane=0.20.6-r3
os/redpanda-25.1:
  - crane=0.20.6-r2
  + crane=0.20.6-r3
2025/08/29 14:24:25 0 builds have new failures
2025/08/29 14:24:25 6 melange builds affected by new packages
```

How reasonable!
Almost nothing uses `crane` in its build environment!
I would feel much more comfortable approving a change to `crane` than I would be to `curl`, wouldn't you?

While we're here, we can demonstrate that local cross-repo deps "just work".
We see `k3s-1.31` in the blast radius, so if I rebuilt it, I would depend on my locally-built `crane` package:

```
make package/k3s-1.31 2>&1 | grep "installing crane"
2025/08/29 14:27:16 INFO installing crane (0.20.6-r3)
```

Indeed, it found the right package and installed my locally-built `crane`.

Eventually, once the cue-ificiation of image plans has fully materialized, we can also look at affected images.

(The output format of all of these commands was arbtirary for the sake of this demo and is not conducive to scripting -- PRs welcome if you, like me, immediately thought "now I want to rebuild and test the whole blast radius".)

### Detecting Unguarded Packages

Another common occurence we'd like to avoid is accidentally orphaning reverse dependencies.
While `stereo impact` can tell you things that _were_ affected by your change, it can't tell you about things that unexpectedly _weren't_.
If I decided to rename `crane` to `crage`, all the package that depend on `crane` would continue to pull in the last-built version.
Sometimes this is easy to detect by just grepping for the package name, but sometimes it's less obvious because things get pulled in by `provides` and `depends` and it's a whole mess.

Let's actually look at that `crane` example by renaming it to `crage`.

We see 0 affected packages, which makes me feel warm and fuzzy (if perhaps slightly confused):

```
stereo impact
2025/08/29 14:41:30 before: saw 7 errors
2025/08/29 14:41:45 after: saw 7 errors
2025/08/29 14:41:45 0 builds have new failures
2025/08/29 14:41:45 0 melange builds affected by new packages
```

But if I look at the output of `stereo unguarded`, I see a handful of packages that still depend on `crane` instead of `crage`:

```
stereo unguarded | grep -B1 crane
enterprise-packages/k3s-1.31:
  crane
--
enterprise-packages/vunnel:
  crane
--
os/k3s:
  crane
--
os/k3s-1.32:
  crane
--
os/neuvector-db:
  crane
--
os/redpanda-25.1:
  crane
```

You'll notice I filtered these results with `grep` because there are a bunch of existing results at HEAD that are unguarded.
Sometimes that's intentional during transition periods, but often it's not!

Eventually, once the cue-ificiation of image plans has fully materialized, we can also look at images depending on unguarded packages.

## Subtree Considerations

An aspirational goal for the migration to stereo is to avoid losing all of the existing commit history.
We can achieve this with [`git-subtree`](https://git.kernel.org/pub/scm/git/git.git/tree/contrib/subtree/git-subtree.adoc), but it has some quirks.

### Committing changes

As part of the migration, we intend to make it straightforward to backport changes from stereo to wolfi, extras, and enterprise.
For those backported commits to make sense, we will require changes that span subtrees to be split into multiple independent commits.
The `git-subtree` docs explain this well:

```
[TIP]
In order to keep your commit messages clean, we recommend that
people split their commits between the subtrees and the main
project as much as possible.  That is, if you make a change that
affects both the library and the main application, commit it in
two pieces.  That way, when you split the library commits out
later, their descriptions will still make sense.  But if this
isn't important to you, it's not *necessary*.  'git subtree' will
simply leave out the non-library-related parts of the commit
when it splits it out into the subproject later.
```

We'll still test these changes together as part of a single PR, but we want to get in the habit of having separate commits for each subtree to avoid awkward commit messages, since we intend to backport these changes to wolfi ~indefinitely.

### Viewing file history

Given how subtrees work, we have to follow merge commits in order for `git log` to work.
Given how often we "merge main into $branch" in wolfi, this makes for a rather unpleasant experience.
You will probably notice that the history view on GitHub is not great.

We're looking at other solutions for this, but in the meantime...

```
git log --full-history --simplify-merges --follow -m -p -- os/crane.yaml
```

This kind of works, but it's hard to follow, sorry.

```
git blame -w -C -M --line-porcelain -- os/crane.yaml \
| awk '/^[0-9a-f]{40} /{print $1}' \
| uniq \
| git log -p --no-merges --no-walk=sorted --stdin --decorate -- crane.yaml
```

This is almost what you'd want but it's also crazy, sorry.

```
git log --follow $(git subtree split --prefix=os) -- crane.yaml
```

This basically works but takes forever to run, sorry.
