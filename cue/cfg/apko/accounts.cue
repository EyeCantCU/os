package apko

import (
	"strconv"
)

// TODO: This could be refactored to clean up downstream consumers

_nonRoot: 65532

// Reusable configuration for users, groups, and accounts to simplify configs
// that need to extend default values.
users: [string]: #User
users: nonroot: {
	username: "nonroot"
	uid:      _nonRoot
	gid:      _nonRoot
}

groups: [string]: #Group
groups: nonroot: {
	groupname: "nonroot"
	gid:       _nonRoot
}

accounts: [string]: #ImageAccounts
accounts: {
	let g = groups
	let u = users

	// Nonroot user and group that runs as nonroot.
	nonroot: {
		"run-as": strconv.FormatInt(_nonRoot, 10)
		groups: [g.nonroot]
		users: [u.nonroot]
	}
}
