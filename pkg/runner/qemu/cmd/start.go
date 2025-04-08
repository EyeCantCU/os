package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/qemu/util"
	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/qemu/util/qmp"
	"github.com/spf13/cobra"
)

func startCmd() *cobra.Command {
	var pubkey, arch string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "start a vm from a vmdir",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			var err error
			pkdata := ""
			if pubkey != "" {
				bpkdata, err := os.ReadFile(pubkey)
				if err != nil {
					log.Fatalf("Failed tor ead public key file from %s", pubkey)
				}
				pkdata = string(bpkdata)
			}
			err = Start(cmd.Context(), args[0], pkdata, types.ParseArchitecture(arch))
			if err != nil {
				log.Fatalf("Error starting vm: %v\n", err)
			}
		},
	}

	cmd.Flags().StringVar(&pubkey, "public-key", "", "id_ed25519")
	cmd.Flags().StringVar(&arch, "arch", runtime.GOARCH, "ARCH")

	return cmd
}

func getSSHPortAddr(ctx context.Context, sockPath string) (string, int, error) {
	mon := &qmp.QMP{Path: sockPath}
	err := mon.Connect(ctx, 10*time.Second)
	if err != nil {
		log.Fatalf("failed connect: %v\n", err)
	}

	defer mon.Close()

	result, err := mon.HumanMonitorCommand("info usernet")
	uninfo, err := util.ParseUsernetInfo(result)
	if err != nil {
		log.Fatalf("parseusernet failed: %v", err)
	}

	for _, i := range uninfo {
		if i.State == "HOST_FORWARD" && strings.HasPrefix(i.SourceAddr, "127.") &&
			i.DestPort == 22 {
			return i.SourceAddr, i.SourcePort, nil
		}
	}

	return "", 0, fmt.Errorf("Did not find ssh port in info usernet\n")
}

func Start(ctx context.Context, dir, pkdata string, arch types.Architecture) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	monitorSock := filepath.Join(dir, "monitor.sock")
	if err := os.Remove(monitorSock); err != nil && !os.IsNotExist(err) {
		return err
	}

	args := util.GetCommand(arch)
	args = append(args,
		[]string{"-smbios", "type=1,product=cgr.dev/qemu/v1",
			"-smbios", "type=11,value=cgr.dev/qemu/v1/ssh-pubkey=" + pkdata}...)

	var sshAddr string
	var sshPort int
	var cmd *exec.Cmd

	cmdChan := make(chan error)
	sshChan := make(chan error)

	go func() {
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = dir
		log.Printf("Starting qemu in %s: %v\n", dir, cmd.Args)
		cmdChan <- cmd.Run()
	}()

	go func() {
		var err error

		sshAddr, sshPort, err = getSSHPortAddr(ctx, filepath.Join(dir, "qmp.sock"))
		sshChan <- err
	}()

	for {
		select {
		case err := <-sshChan:
			if err != nil {
				return err
			}
			log.Printf("ssh available on %s:%d\n", sshAddr, sshPort)
		case err := <-cmdChan:
			return err
		}
	}
}
