package sshutils

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
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
	return tmpFile.Name(), nil
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

// GetPrivateKeyOrDefaultAuth - return AuthMethod to for ssh.
//
// If path is empty, then use GetSigners() otherwise return a PrivateKeyAuth
// for the provided path.
func GetPrivateKeyOrDefaultAuth(path string) ([]ssh.AuthMethod, error) {
	if path == "" {
		return GetSigners()
	}
	return PrivateKeyAuth([]string{path})
}

// GetSigners - returns a slice of ssh.AuthMethod easily used ClientConfig.Auth
func GetSigners() ([]ssh.AuthMethod, error) {
	ret := []ssh.AuthMethod{}

	pubkeys := []string{}
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
			pubkeys = append(pubkeys, string(ssh.MarshalAuthorizedKey(k)))
		}
		ret = append(ret, ssh.PublicKeysCallback(ag.Signers))
	}

	userKeys, err := getUserPrivateKeys()
	if err != nil {
		return ret, err
	}

	for _, k := range userKeys {
		if !slices.Contains(pubkeys, string(ssh.MarshalAuthorizedKey(k.PublicKey()))) {
			ret = append(ret, ssh.PublicKeys(k))
		}
	}

	return ret, nil
}

func getUserPrivateKeys() ([]ssh.Signer, error) {
	ret := []ssh.Signer{}
	user, err := user.Current()
	if err != nil {
		return ret, err
	}
	keydir := filepath.Join(user.HomeDir, ".ssh")

	for _, fname := range []string{"id_ed25519", "id_rsa"} {
		fpath := filepath.Join(keydir, fname)
		key, err := os.ReadFile(fpath)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return ret, fmt.Errorf("Error reading %s: %v", fpath, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return ret, fmt.Errorf("Error parsing %s: %v", fpath, err)
		}
		ret = append(ret, signer)
	}
	return ret, nil
}

// GetUserPubkeys - return public keys suitable for authorized_keys
//
// Search common path locations and consult ssh-agent if present.
func GetUserPubkeys() ([]string, error) {
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

// PrivateKeyAuth - return slice of ssh.PublicKeys for the provided paths.
func PrivateKeyAuth(paths []string) ([]ssh.AuthMethod, error) {
	ret := []ssh.AuthMethod{}
	for _, p := range paths {
		key, err := os.ReadFile(p)
		if err != nil {
			return ret, fmt.Errorf("unable to read private key: %w", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return ret, fmt.Errorf("unable to parse private key: %w", err)
		}
		ret = append(ret, ssh.PublicKeys(signer))
	}
	return ret, nil
}

// SSHCommand - run ssh command (for interactive use)
func SSHCommand(ctx context.Context, host, user, privateKeyPath, knownHostsFile string, args []string) {
	hostOnly, port, err := net.SplitHostPort(host)
	if err != nil {
		port = "22"
		hostOnly = host
	}

	opts := []string{}
	if privateKeyPath != "" {
		opts = append(opts, "-i"+privateKeyPath)
	}

	opts = append(opts, "-p"+port, "-oUserKnownHostsFile="+knownHostsFile)
	opts = append(opts, fmt.Sprintf("%s@%s", user, hostOnly))
	opts = append(opts, args...)
	execSSH := exec.CommandContext(ctx, "ssh", opts...)
	execSSH.Stdout = os.Stdout
	execSSH.Stderr = os.Stderr
	execSSH.Stdin = os.Stdin
	execSSH.Run()
}

// NewSSHClient takes a host, ssh key, user, and knownHostsFile, and returns an open ssh client connection.
func NewSSHClient(ctx context.Context, host string, auths []ssh.AuthMethod, user, knownHostsFile string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            auths,
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
func SSHToInstance(ctx context.Context, host string, auths []ssh.AuthMethod, user, knownHostsFile string, command []string) error {
	client, err := NewSSHClient(ctx, host, auths, user, knownHostsFile)
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
	fmt.Printf("%s", output)
	return nil
}

// ShoveBinaryFile takes a host, ssh key, user, knownHostsFile, local file path, and remote destination.
func ShoveBinaryFile(ctx context.Context, host string, auths []ssh.AuthMethod, user, knownHostsFile, localFilePath, destFilePath string) error {
	client, err := NewSSHClient(ctx, host, auths, user, knownHostsFile)
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

// GetSinglePublicKey - get a public key suitable for authorized_keys.
//
//	if path is empty string, then use GetUserPubkeys to find user public
//	keys and return the first in the list.
func GetSinglePublicKey(path string) (string, error) {
	if path != "" {
		pubKeyBytes, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("failed to read public key: %v", err)
		}
		return string(pubKeyBytes), nil
	}

	pubkeys, err := GetUserPubkeys()
	if err != nil {
		return "", fmt.Errorf("failed to get public keys (try --public-key): %v", err)
	}
	if len(pubkeys) == 0 {
		return "", fmt.Errorf("Not able to find a default public key. Try --public-key")
	} else if len(pubkeys) > 1 {
		log.Printf("%d public keys were found. selected first one", len(pubkeys))
	}
	log.Printf("Using public key: %s", pubkeys[0])
	return pubkeys[0], nil
}

// Guess the path to the home directory for the given remote user.
func GuessHomePath(user string) string {
	if user == "root" {
		return "/root"
	} else {
		return "/home/" + user
	}
}
