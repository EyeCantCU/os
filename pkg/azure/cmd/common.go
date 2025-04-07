package cmd

import "fmt"

func toPtr[T any](v T) *T {
	return &v
}

func getSSHPath(user string) string {
	if user == "root" {
		return "/root/.ssh/authorized_keys"
	}
	return fmt.Sprintf("/home/%s/.ssh/authorized_keys", user)
}
