// Copyright 2022 Chainguard, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strings"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/utils"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func debugCmd() *cobra.Command {
	var arch string
	var ovmf string
	var pubkey string

	cmd := &cobra.Command{
		Use:     "debug",
		Short:   "Debug a vm disk",
		Example: `  wolfi-vm debug [disk.rawl]`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var efiDisk string
			if len(args) > 0 {
				efiDisk = args[0] // e.g. "disk.raw"
			}

			if efiDisk == "" {
				return fmt.Errorf("empty config, specify valid raw disk path")
			}

			if arch == "" {
				return fmt.Errorf("please specify target VM architecture")
			}

			// standardize everywhere
			arch := types.ParseArchitecture(arch).ToAPK()

			return DebugCmd(ctx, efiDisk, arch, ovmf, pubkey)
		},
	}

	cmd.Flags().StringVarP(&pubkey, "pubkey", "p", "auto", "public key to supply to vm")
	cmd.Flags().StringVar(&arch, "arch", "", "arch to build the disk")
	cmd.Flags().StringVar(&ovmf, "ovmf", "", "uefi file to boot")

	return cmd
}

func getUserPubkeys() ([]string, error) {
	ret := []string{}
	user, err := user.Current()
	if err != nil {
		return ret, err
	}
	keydir := filepath.Join(user.HomeDir, ".ssh")

	for _, fname := range []string{"id_ed25519.pub", "id_rsa.pub"} {
		fpath := filepath.Join(keydir, fname)
		pubkeyb, err := os.ReadFile(fpath)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return ret, fmt.Errorf("Error reading %s: %v", fpath, err)
		}
		ret = append(ret, string(pubkeyb))
	}

	if len(ret) != 0 {
		return ret, nil
	}

	if authSock := os.Getenv("SSH_AUTH_SOCK"); authSock != "" {
		sshAgent, err := net.Dial("unix", authSock)
		if err != nil {
			return ret, fmt.Errorf("failed to connect to SSH_AUTH_SOCK=%s: %v", authSock, err)
		}
		ag := agent.NewClient(sshAgent)

		aKeys, err := ag.List()
		if err != nil {
			return ret, fmt.Errorf("Error listing public keys in SSH_AUTH_SOCK=%s: %v", authSock, err)
		}

		for _, k := range aKeys {
			pk := string(ssh.MarshalAuthorizedKey(k))
			if !slices.Contains(ret, pk) {
				ret = append(ret, pk)
			}
		}
	}

	return ret, nil
}

func DebugCmd(ctx context.Context, efiDisk, arch, ovmf, pubkey string) error {
	if ovmf == "" {
		ovmfDir, err := os.MkdirTemp("", "")
		if err != nil {
			return fmt.Errorf("os.MkdirTemp() failed with %w", err)
		}

		ovmf, err = utils.FetchBios(ovmfDir, arch)
		if err != nil {
			return err
		}

		defer os.RemoveAll(ovmfDir)
	}

	var pubkeys = ""
	if pubkey == "none" || pubkey == "" {
	} else if pubkey == "auto" {
		keys, err := getUserPubkeys()
		if err != nil {
			log.Fatalf("failed to get user's public keys: %v", err)
		}
		pubkeys = strings.Join(keys, "\n")
	} else {
		content, err := os.ReadFile(pubkey)
		if err != nil {
			log.Fatalf("failed to read public key '%s': %v", pubkey, err)
		}
		pubkeys = string(content)
	}

	args := utils.GenerateQEMUCommand(arch, efiDisk, ovmf)

	if pubkeys != "" || pubkey == "none" {
		args = append(args, []string{
			"-smbios", "type=1,product=cgr.dev/qemu/v1",
			"-smbios", "type=11,value=cgr.dev/qemu/v1/ssh-pubkey=" + pubkeys}...)
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	return cmd.Run()
}
