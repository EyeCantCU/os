package main

import (
	"strings"
	"list"
	artv1 "chainguard.dev/wolfi-vm/cue/cfg/art/v1"
)

// art defines all of the images and image groups for a package. It is consumed
// by the `art` CLI.
art: artv1.#Inventory

// pkgLocks is a map of image identifiers defined in art to the resolved
// packages for each image architecture + dev variant
pkgLocks: artv1.#PackageLocks

// imgLocks is a map of image identifiers defined in art to the fully locked
// `apko` image configurations.
imgLocks: artv1.#ImageLocks

art: {
	// Decorate all images with defaults based on #Config
	images: [string]: artv1.#Image
	for k, v in images
	// groups: [string]: artv1.#ImageGroup
	// for k, v in groups {
	// 	groups: (k): {
	// 		for c, cv in v.components {
	// 			components: (c): _img & {
	// 				_id: c
	// 				metadata: fips: strings.HasSuffix(k, "-fips")
	// 			} & cv
	// 		}
	// 	}
	// }
	{
		images: (k): _img & {_id: k} & v
	}
}

// _img decorates an #Image with default values and values from artConfig and
// stores the result on .
// It is applied to each image in our #Inventory via unification.
_img: img={
	_id: string // The image identifier / map key

	main: string | *""
	pkgs: img.pkgs

	config: {
		contents: {
			packages: list.Concat([
				artConfig.contents.packages,
				[
					if main != "" {[main]},
					[],
				][0],
				[if metadata.fips {fipsDefaultPkgs}, nonFipsDefaultPkgs][0],
				[
					if img.filesystem != "" {
						["systemd-repart-rootfs-"+img.filesystem]
					},
					[]
				][0],
				pkgs,
			])
			build_repositories:   artConfig.contents.build_repositories
			runtime_repositories: artConfig.contents.runtime_repositories
		}
		archs: img.config.archs | *artConfig.archs
		// accounts: {
		// 	"run-as": string | *"65532"
		// 	users: img.config.accounts.users | *[apko.users.nonroot]
		// 	groups: img.config.accounts.groups | *[apko.groups.nonroot]

		// 	// Decorate groups with deeper defaults
		// 	groups: [
		// 		for g in groups
		// 		let _gid = g.gid
		// 		let _groupname = g.groupname {
		// 			gid: [
		// 				if _gid != _|_ {_gid},
		// 				65532,
		// 			][0]
		// 			groupname: [
		// 				if _groupname != _|_ {_groupname},
		// 				"nonroot",
		// 			][0]
		// 		},
		// 	]

		// 	// Decorate users with deeper defaults
		// 	users: [
		// 		for u in users
		// 		let _uid = u.uid
		// 		let _gid = u.gid
		// 		let _username = u.username {
		// 			uid: [
		// 				if _uid != _|_ {_uid},
		// 				65532,
		// 			][0]
		// 			gid: [
		// 				if _gid != _|_ {_gid},
		// 				65532,
		// 			][0]
		// 			username: [
		// 				if _username != _|_ {_username},
		// 				"nonroot",
		// 			][0]
		// 		},
		// 	]
		// }
	}

	dev: img.dev | *{enabled: true}

	metadata: {
		// If we set metadata.fips: strings.Contains(_id, "fips"), being named "fips" would be the only way to control metadata.fips.
		// We want images to be able to turn fips setting to true regardless of naming. (aws-atsec-jitterentropy-fips-full does this).
		if strings.Contains(_id, "fips") {
			fips: true
		}
	}
}
