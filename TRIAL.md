# Trial

Welcome to stereo!
Thank you for your willingness to participate in this experiment.

## Workflow

During the trial, we won't actually be merging any package changes into stereo.
Because we'll sync all package defintions from wolfi, extras, and enterprise into stereo directly, merging package changes into stereo would cause conflicts.
Instead, we'll just be opening Pull Requests, observing the results of the CI, and closing them.

Your job is to evaluate the effectiveness of stereo's CI to catch common failures that we would otherwise miss.

## Getting Started

We fully intend to automate all of this, but currently things are quite janky.

Before making any changes, you'll want to sync upstream sources into stereo:

```
./hack/sync.sh
```

Then we'll make a small change, e.g. we can simulate adding a new enterprise package that already exists in wolfi:

```
cp os/crane.yaml enterprise-packages/
```

One of our goals is to prevent us from accidentally doing this.
Let's see if `stereo lint` catches that:

```
go run ./cmd/stereo lint
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
