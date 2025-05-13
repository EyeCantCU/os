package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/utils/sshutils"
	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/qemu/util"
	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/qemu/util/qmp"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
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
			if pubkey == "" {
				pubkeys, err := sshutils.GetUserPubkeys()
				fmt.Printf("pubkeys: %v\n", pubkeys)
				if err != nil {
					log.Fatalf("Could not find default public key. try --public-key: %v", err)
				}
				pkdata = strings.Join(pubkeys, "\n")
			} else if pubkey != "none" {
				bpkdata, err := os.ReadFile(pubkey)
				if err != nil {
					log.Fatalf("Failed tor ead public key file from %s", pubkey)
				}
				pkdata = string(bpkdata)
			}
			err = Start(cmd.Context(), args[0], pkdata, types.ParseArchitecture(arch))
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	cmd.Flags().StringVar(&pubkey, "public-key", "", "id_ed25519")
	cmd.Flags().StringVar(&arch, "arch", runtime.GOARCH, "ARCH")

	return cmd
}

func sigStr(sig os.Signal) string {
	sigstr := sig.String()
	if s, ok := sig.(syscall.Signal); ok {
		sigstr = unix.SignalName(s)
	}
	return sigstr
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
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	monitorSock := filepath.Join(dir, "monitor.sock")
	if err := os.Remove(monitorSock); err != nil && !os.IsNotExist(err) {
		return err
	}

	args := util.GetCommand(arch)
	args = append(args,
		[]string{"-smbios", "type=1,product=cgr.dev/qemu/v1",
			"-smbios", "type=11,value=cgr.dev/qemu/v1/ssh-pubkey=" + pkdata}...)

	sigs := make(chan os.Signal, 1)
	var sigReceived os.Signal
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	// kill with SIGTERM rather than SIGKILL
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			log.Printf("context canceled but pid cmd.Process is nil")
			return nil
		}
		log.Printf("sending %s to %s pid %d", sigStr(sigReceived), args[0], cmd.Process.Pid)
		cmd.Process.Signal(sigReceived)
		_, err := cmd.Process.Wait()
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		select {
		case sigReceived = <-sigs:
			log.Printf("%s received. shutting down", sigStr(sigReceived))
			cancel(fmt.Errorf("%s received", sigStr(sigReceived)))
		case <-ctx.Done():
		}
	}()

	go func() {
		qmpSock := filepath.Join(dir, "qmp.sock")
		sshAddr, sshPort, err := getSSHPortAddr(ctx, qmpSock)
		if err != nil {
			if ctx.Err() != nil {
				cancel(fmt.Errorf("failed to get ssh connection info: %v", err))
			}
			return
		}
		log.Printf("%s is pid %d. qmp-socket in %s\n", args[0], cmd.Process.Pid, qmpSock)
		log.Printf("ssh available on %s:%d\n", sshAddr, sshPort)
	}()

	err := cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
