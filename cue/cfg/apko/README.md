## Updating `types.cue`

`types.cue` is _mostly_ generated from upstream Go source for `apko` here: https://github.com/chainguard-dev/apko.

In the event that `apko/pkg/build/types` changes, here is a rough guide on how to update those types:

- Navigate to up-to-date clone of `github.com/chainguard-dev/apko`
- Remove the `MarshalYAML` and `UnmarshalYAML` functions from `pkg/build/types/types.go` (this causes CUE to assume that the structs defined in Go are not authoritative, resulting in a `_`)
- `cue get go ./pkg/build/types`
- Apply the following changes to the generated `cue.mod/gen/chainguard.dev/apko/pkg/build/types/types_gen_go.cue`:
  - Inline `#Hash` so that we don't need google/go-containerregistry:
    ```
    #Hash: {
	    Algorithm: string
  	  Hex:       string
    }
    ```
  - Update references to `v1.#Hash` with `#Hash`
  - Remove import for google/go-containerregistry
  - Change package name to `apko`
  - Add additional constraint for `#Architecture`:
    ```
    #Architecture: "amd64" | "arm64"
    ```
  - Make `#User.username`, `#User.uid`, and `#User.gid` required by replacing `?` with `!` on each identifier.
- Copy the new types back here: `cp cue.mod/gen/chainguard.dev/apko/pkg/build/types/types_gen_go.cue ../../chainguard-images/images-private/x/cue/cfg/apko/types.cue`

>NOTE: These types changes so rarely and in small ways that it might just be easier to manually update them over time. Otherwise, we should eventually maintain patches that can be applied programmatically to smooth out this process.
