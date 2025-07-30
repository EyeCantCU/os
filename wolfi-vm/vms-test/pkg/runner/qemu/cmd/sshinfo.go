package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/qemu/util"
	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/qemu/util/qmp"
	"github.com/spf13/cobra"
)

func sshInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh-info",
		Short: "print ssh connection information in format addr:port",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			vmd := args[0]
			sockPath := filepath.Join(vmd, "qmp.sock")

			mon := &qmp.QMP{Path: sockPath}
			err := mon.Connect(cmd.Context(), 10*time.Second)
			if err != nil {
				log.Fatalf("failed connect: %v\n", err)
			}

			defer mon.Close()

			result, err := mon.Send(qmp.Command{Execute: "human-monitor-command",
				Arguments: map[string]string{"command-line": "info usernet"}})
			var rstr string
			if err := json.Unmarshal(result.Return, &rstr); err != nil {
				log.Fatalf("rstr didn't work: %v\n", err)
			}

			uninfo, err := util.ParseUsernetInfo(rstr)
			if err != nil {
				log.Fatalf("parseusernet failed: %v", err)
			}

			port := 0
			addr := ""
			for _, i := range uninfo {
				if i.State == "HOST_FORWARD" && strings.HasPrefix(i.SourceAddr, "127.") &&
					i.DestPort == 22 {
					port = i.SourcePort
					addr = i.SourceAddr
					break
				}
			}
			fmt.Printf("%s:%d\n", addr, port)
		},
	}

	return cmd
}
