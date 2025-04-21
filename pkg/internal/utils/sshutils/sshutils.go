package sshutils

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// NewSSHClient takes a host, ssh key, and user and returns an open ssh client connection.
func NewSSHClient(ctx context.Context, host, privateKeyPath, user string) (*ssh.Client, error) {
	key, err := ioutil.ReadFile(privateKeyPath)
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: implement TOFU or fetch the keys out of band or something?
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

// SSHToInstance takes a host, ssh key, user, and optional command. If command
// is empty, it runs exec ssh with appropriate arguments. Otherwise, runs the command
// remotely and prints the output.
func SSHToInstance(ctx context.Context, host, privateKeyPath, user string, command []string) error {
	if len(command) > 0 {
		client, err := NewSSHClient(ctx, host, privateKeyPath, user)
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
			"-p" + port, "-i", privateKeyPath,
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

// ShoveBinaryFile takes a host, ssh key, user, local file path, and remote destination. It copies the
// file to the remote host, marks it executable, and returns any error.
func ShoveBinaryFile(ctx context.Context, host, privateKeyPath, user, localFilePath, destFilePath string) error {
	client, err := NewSSHClient(ctx, host, privateKeyPath, user)
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
