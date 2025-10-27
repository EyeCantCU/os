package utils

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var shellsafe = regexp.MustCompile("^[a-zA-Z0-9-/:=_.,]+$")

// PrintFormat - format this []string in a way that is mostly shell friendly and readable
func PrintFormat(args []string) string {
	quoted := []string{}
	for _, arg := range args {
		if shellsafe.MatchString(arg) {
			quoted = append(quoted, arg)
		} else {
			quoted = append(quoted, strconv.Quote(arg))
		}
	}
	return strings.Join(quoted, " ")
}

// CmdPrintFormat - format this exec.Cmd in a way that is mostly shell friendly and readable
func CmdPrintFormat(cmd *exec.Cmd) string {
	return PrintFormat(cmd.Args)
}
