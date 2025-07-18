package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/siderolabs/go-smbios/smbios"
)

const product = "cgr.dev/qemu/v1"
const Usage = `Usage: qemu-guesthelp subcommand [args]

   subcommands:

    * fetch-keys user

	  fetch keys from smbios for user`

func fetchKeys(user string, writer io.Writer) error {
	// -smbios "type=1,product=cgr.dev/qemu/v1"
	// -smbios "type=11,value=cgr.dev/qemu/v1/ssh-pubkey=$pk"
	b, err := smbios.New()
	if err != nil {
		return err
	}

	if b.SystemInformation.ProductName != product {
		return nil
	}

	keyname := product + "/ssh-pubkey"

	for _, s := range b.OEMStrings.Strings {
		toks := strings.SplitN(s, "=", 2)
		if len(toks) != 2 || toks[0] != keyname {
			continue
		}

		// user:key or key
		key := toks[1]
		keytoks := strings.SplitN(toks[1], ":", 2)
		if len(keytoks) == 2 {
			// user:pubkey format
			if keytoks[1] != user {
				continue
			}
		}

		if _, err := writer.Write([]byte(key + "\n")); err != nil {
			return fmt.Errorf("Failed writing to output: %v", err)
		}
	}
	return nil
}

func main() {
	fetchKeysCmd := flag.NewFlagSet("fetch-keys", flag.ExitOnError)

	if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Println(Usage)
		os.Exit(0)
	}

	// The subcommand is expected as the first argument
	// to the program.
	if len(os.Args) < 2 {
		log.Fatal(Usage)
	}

	// Check which subcommand is invoked.
	switch os.Args[1] {
	case "fetch-keys":
		fetchKeysCmd.Parse(os.Args[2:])
		if fetchKeysCmd.NArg() != 1 {
			log.Fatalf("Error got %d args (%v) expected 1\n", fetchKeysCmd.NArg(), fetchKeysCmd.Args())
		}
		user := fetchKeysCmd.Args()[0]
		if err := fetchKeys(user, os.Stdout); err != nil {
			log.Fatalf("Error fetching keys: %v", err)
		}
		os.Exit(0)
	default:
		log.Fatal(Usage)
	}
}
