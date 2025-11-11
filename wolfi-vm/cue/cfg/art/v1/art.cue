package v1

import (
	"strings"
	"strconv"
	"path"
	"list"
	"chainguard.dev/wolfi-vm/cue/cfg/apko"
)

// #Config exposes configuration for controlling `art`'s behavior.
#Config: {
	// Image contents that are added to all image configurations.
	contents: {
		apko.#ImageContents
		devPackages?: [...string]
	}

	// Default architectures for images which don't explicitly define supported
	// architectures. At least one must be provided
	archs?: [...apko.#Architecture]
	archs: list.MinItems(1)

	// Define apko configurations that should be used if not explicitly defined
	// in an image.
	defaults: {
		accounts: apko.#ImageAccounts
	}

	// OCI repositories to publish images to.
	ociRepository:       string
	ociCustomRepository: string | *ociRepository
}

// #Inventory is the full set of images defined by a CUE module. A valid module
// must expose one and only one field of this type to be valid.
#Inventory: {
	// images are standalone images, which can be built, tested, and released
	// independently.
	images: [string]: #Image
	// groups are collections of images which are deployed and optinally versioned
	// together. A group's tests may rely on standalone images.
	groups: [string]: #ImageGroup
}

// Definition for each individual image a package produces.
#Image: {
	// The 'main' package for an image. If set, version-based tags and EOL
	// information will be generated.
	main?: string
	// The version information for this image, if 'main' was set and this is a
	// version streamed image.
	version?: #SemVer
	// Additional non-main packages.
	pkgs?: [...string]
	// Additional specific apko configuration for the image.
	config: #ImageConfiguration
	// Dev variant configuration
	dev?: {
		enabled: bool | *true
		// pkgs are additional packages added to dev variant of the image
		pkgs?: [...string]
	}

	// Filesystem  for systemd-repart-rootfs-*.
	// set to "" to remove this package.
	filesystem: string | *"xfs"

	// TODO: do this for real from source of truth gotypes
	metadata: {
		fips: bool | *false
		// The origin field overrides the origin package for the image when it differs
		// from the main package. e.g. kubectl's origin package is kubernetes.
		origin?: string
	}

	// repo is the destination repository fragment for the image, e.g. "harbor-db".
	// It is combined with the values from #Config to produce the full destination
	// repo URI (e.g. foo.bar/baz/$repo).
	ociRepository?: string

	// NOTE: This isn't wired up, its just a sketch of how test config _could_
	// 			 be represented
	tests?: #ImageTests

	// tags controls non-default tagging behavior.
	tags?: {
		// Template strings that are used to create additional tags that are applied
		// during publishing in addition to the standard version tags generated from
		// the resolving pkg.
		templates?: [...string]
		// Explicitly control whether or not this image should be tagged as latest,
		// for situations where it can't be reliably determined from package name
		// (e.g. tomcat-11-openjdk-21 vs tomcat-11-openjdk-17)
		latest?: bool | *false
	}
}

#ImageGroup: {
	// TODO: allow setting metadata and friends?
	components: [string]: #Image
	tests?: #ImageTests
}

// #ImageConfiguration is an apko image configuration (apko.#ImageConfiguration)
// with an additional field that allows referencing the resolved packages for
// an image configuration by the requested package name. `resolvedPkgs` is
// populated by `art` while locking the image configuration.
#ImageConfiguration: {
	apko.#ImageConfiguration
	resolvedPkgs?: #ResolvedPkgs
}

// #ResolvedPkgs is embedded into #ImageConfiguration to allow creating configs
// that reference information about the image's resolved packages,
// e.g. `environment.APP_VERSION`.
#ResolvedPkgs: {
	byName: [string]:     #PackageVersion
	byProvides: [string]: #PackageVersion
}

#ImageTests: {
	// Terraform offramp that invokes tf directly
	tf?: {
		path:      string | *"./tests"
		variables: _
	}
	// Standalone imagetest binary, or other non-TF test runner
	imagetest: _
}

// #PackageLocks represents the full collection of #PackageLock in an art
// gallery. The elements can either be a single #PackageLock, or a set
// corresponding to an #ImageGroup's locks.
#PackageLocks: {
	images: [string]: #PackageLock
	groups: [string]: components: [string]: #PackageLock
}

// #PackageLock represents a specific package resolution for a specific
// architecture or dev variant.
#PackageLock: {
	// pkgs is either the list of resolved packages (when all architecture resolve
	// to the same packages) or a map of architectures to their specific packages:
	//
	// 	{
	//		index: [a, b, c, d]
	//		amd64: [f] // The full list is index + amd64
	//	}
	pkgs: [...string] | {[string]: [...string]}
	dev: [...string] | {[string]: [...string]}
	// byName allows referencing resolved `pkgs` by the name they were requested by
	byName: [string]: #PackageVersion
	// byProvides allows referencing resolved 'pkgs' by the name of their provides
	byProvides: [string]: #PackageVersion
}

#ImageLocks: {
	images: [string]: #ImageLock
	groups: [string]: components: [string]: #ImageLock
}

// TODO: trim this down to bare essentials
#ImageLock: {
	configs: [string]:     apko.#ImageConfiguration
	devConfigs?: [string]: apko.#ImageConfiguration
	// TODO: ociRepository: #OCIRef
	ociRepository: string
	main?:         string
	latest?:       bool
	eol?:          bool
	tags?: [...string]
}

// TODO: update this to allow partial semver strings (11.1)
_#validSemVer: string & =~"^\\d+\\.\\d+\\.\\d+(-[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?(\\+[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$"

// #SemVer represents a valid package semver string, deconstructed into its
// individual components. Valid partials are accepted. A #SemVer may optionally
// contain information about the version stream, if it belongs to one.
#SemVer: {
	// TODO: use _#validSemVer when it works
	version!: string
	stream?: {
		name!:  string
		eol:    bool | *false
		latest: bool | *false
	}

	let t = strings.Split(strings.Split(version, "-")[0], ".")
	major: strconv.Atoi(t[0])
	minor: *(strconv.Atoi(t[1])) | 0
	patch: *(strconv.Atoi(t[2])) | 0
}

#PackageVersion: {
	version!:     string
	withoutEpoch: strings.Split(version, "-")[0]
	epoch:        strings.Split(version, "-")[1]

	parts: strings.Split(withoutEpoch, ".")

	major: *(strconv.Atoi(parts[0])) | 0
	minor: *(strconv.Atoi(parts[1])) | 0
	patch: *(strconv.Atoi(parts[2])) | 0
}

#OCIRef: {
	repository!: string

	let t = strings.SplitN(repository, "/", 1)
	registry: t[0]
	repoName: path.Base(t[1])

	ref:    string
	tag:    string & strings.MaxRunes(128)
	digest: string

	if digest != "" && tag != "" {
		ref: "\(repository):\(tag)@\(digest)"
	}
	if digest != "" && tag == "" {
		ref: "\(repository)@\(digest)"
	}
	if digest == "" && tag != "" {
		ref: "\(repository):\(tag)"
	}
	if digest == "" && tag == "" {
		ref: "\(repository):\(tag)"
	}
}
