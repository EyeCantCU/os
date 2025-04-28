package sshutils

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	// TODO: mutex by filename. No usecase yet though.
	knownHostsMu sync.Mutex
)

// Return a path to an ephemeral known hosts file. Caller is responsible for cleanup.
func EphemeralKnownHosts() (string, error) {
	tmpFile, err := os.CreateTemp(os.TempDir(), "known_hosts")
	if err != nil {
		return "", fmt.Errorf("failed to make ephemeral known hosts file: %w", err)
	}
	defer tmpFile.Close()
	return filepath.Join(os.TempDir(), tmpFile.Name()), nil
}

// TOFUHostKeyCallback implements trust on first use, storing known host keys
// in the standard OpenSSH known_hosts format.
func TOFUHostKeyCallback(knownHostsFile, host string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		knownHostsMu.Lock()
		defer knownHostsMu.Unlock()

		data, _ := os.ReadFile(knownHostsFile)
		knownHosts := make(map[string]ssh.PublicKey)
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			hostnames := strings.Split(fields[0], ",")
			keyType := fields[1]
			keyData := fields[2]
			for _, h := range hostnames {
				pubKeyBytes, err := base64.StdEncoding.DecodeString(keyData)
				if err != nil {
					continue
				}
				pubKey, err := ssh.ParsePublicKey(pubKeyBytes)
				if err != nil {
					continue
				}
				// Verify type matches
				if pubKey.Type() == keyType {
					knownHosts[h] = pubKey
				}
			}
		}

		if existingKey, ok := knownHosts[host]; ok {
			if ssh.FingerprintSHA256(existingKey) != ssh.FingerprintSHA256(key) {
				return fmt.Errorf("host key mismatch for %s", host)
			}
		} else {
			f, err := os.OpenFile(knownHostsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return fmt.Errorf("unable to write host key: %w", err)
			}
			defer f.Close()

			line := fmt.Sprintf("%s %s %s\n", host, key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))
			if _, err := f.WriteString(line); err != nil {
				return fmt.Errorf("unable to write known_hosts line: %w", err)
			}
		}

		return nil
	}
}

// NewSSHClient takes a host, ssh key, user, and knownHostsFile, and returns an open ssh client connection.
func NewSSHClient(ctx context.Context, host, privateKeyPath, user, knownHostsFile string) (*ssh.Client, error) {
	key, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: TOFUHostKeyCallback(knownHostsFile, host),
		Timeout:         10 * time.Second,
	}

	if !regexp.MustCompile(`:[0-9]+$`).MatchString(host) {
		host = fmt.Sprintf("%s:22", host)
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, host, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect via ssh: %w", err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// SSHToInstance takes a host, ssh key, user, knownHostsFile, and optional command.
func SSHToInstance(ctx context.Context, host, privateKeyPath, user, knownHostsFile string, command []string) error {
	if len(command) > 0 {
		client, err := NewSSHClient(ctx, host, privateKeyPath, user, knownHostsFile)
		if err != nil {
			return fmt.Errorf("failed to make ssh client: %w", err)
		}
		defer client.Close()
		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
		defer session.Close()

		cmdString := strings.Join(command, " ")
		output, err := session.CombinedOutput(cmdString)
		if err != nil {
			return fmt.Errorf("command error: %w\nOutput: %s", err, output)
		}
		fmt.Print(string(output))
	} else {
		hostOnly, port, err := net.SplitHostPort(host)
		if err != nil {
			port = "22"
			hostOnly = host
		}

		sshargs := []string{
			"-p" + port, "-i", privateKeyPath, "-o", "UserKnownHostsFile=" + knownHostsFile,
			fmt.Sprintf("%s@%s", user, hostOnly),
		}
		execSSH := exec.CommandContext(ctx, "ssh", sshargs...)
		execSSH.Stdout = os.Stdout
		execSSH.Stderr = os.Stderr
		execSSH.Stdin = os.Stdin
		execSSH.Run()
	}

	return nil
}

// ShoveBinaryFile takes a host, ssh key, user, knownHostsFile, local file path, and remote destination.
func ShoveBinaryFile(ctx context.Context, host, privateKeyPath, user, knownHostsFile, localFilePath, destFilePath string) error {
	client, err := NewSSHClient(ctx, host, privateKeyPath, user, knownHostsFile)
	if err != nil {
		return fmt.Errorf("failed to make ssh client: %w", err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	f, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	session.Stdin = f
	cmd := fmt.Sprintf("cat > %s && chmod +x /%s", destFilePath, destFilePath)
	output, err := session.Output(cmd)
	if err != nil {
		return fmt.Errorf("failed to shove file: %w\nOutput: %s", err, output)
	}
	return nil
}

// GetAuthorizedKeysPath takes a username and returns the correct path to the authorized_keys file
func GetAuthorizedKeysPath(user string) string {
	if user == "root" {
		return "/root/.ssh/authorized_keys"
	}
	return fmt.Sprintf("/home/%s/.ssh/authorized_keys", user)
}

// WaitForSSHHostKey tries to retrieve the SSH host key from the target address within the given timeout.
// It retries at intervals specified by retryInterval.
func WaitForSSHHostKey(parentCtx context.Context, address string, retryInterval time.Duration) (ssh.PublicKey, error) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	if !regexp.MustCompile(`:[0-9]+$`).MatchString(address) {
		address = fmt.Sprintf("%s:22", address)
	}

	var lastErr error

	for {
		hostKey, err := GetSSHHostKey(ctx, address)
		if err == nil {
			return hostKey, nil
		}

		if ctx.Err() == nil {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			err := ctx.Err()
			reason := "error"
			if errors.Is(err, context.DeadlineExceeded) {
				reason = "timed-out"
			} else if errors.Is(err, context.Canceled) {
				reason = "canceled"
			}

			return nil, fmt.Errorf("context %s. last error: %v: %w", reason, lastErr, err)
		case <-time.After(retryInterval):
		}
	}
}

// GetSSHHostKey connects and extracts the SSH host key without authenticating.
func GetSSHHostKey(ctx context.Context, address string) (ssh.PublicKey, error) {
	result := make(chan ssh.PublicKey, 1)
	errs := make(chan error, 1)

	go func() {
		var serverHostKey ssh.PublicKey

		dialer := net.Dialer{Timeout: 5 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			errs <- err
			return
		}
		defer conn.Close()

		config := &ssh.ClientConfig{
			User: "invalid",
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				serverHostKey = key
				return nil // we accept it for observation
			},
			Timeout: 5 * time.Second,
		}

		sshConn, _, _, err := ssh.NewClientConn(conn, address, config)
		if err != nil {
			if serverHostKey != nil {
				result <- serverHostKey
			} else {
				errs <- err
			}
			return
		}
		sshConn.Close()

		errs <- fmt.Errorf("host key was not received. but connection succeeded!?")
		return

	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("GetSSHHostKey: context expired before host key could be retrieved: %w", ctx.Err())
	case err := <-errs:
		return nil, err
	case pub := <-result:
		return pub, nil
	}
}
