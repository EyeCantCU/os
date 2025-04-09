package sshutils

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
	"regexp"

	"golang.org/x/crypto/ssh"
)

// NewSSHClient takes a host, ssh key, and user and returns an open ssh client connection.
func NewSSHClient(host, privateKeyPath, user string) (*ssh.Client, error) {
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
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	return client, nil
}

// SSHToInstance takes a host, ssh key, user, and optional command. If command
// is empty, it opens an interactive terminal session. Otherwise, runs the command
// remotely and prints the output.
func SSHToInstance(host, privateKeyPath, user string, command []string) error {
	client, err := NewSSHClient(host, privateKeyPath, user)
	if err != nil {
		return fmt.Errorf("failed to make ssh client: %w", err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	if len(command) > 0 {
		cmdString := strings.Join(command, " ")
		output, err := session.CombinedOutput(cmdString)
		if err != nil {
			return fmt.Errorf("command error: %w\nOutput: %s", err, output)
		}
		fmt.Print(string(output))
	} else {
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr
		session.Stdin = os.Stdin
		modes := ssh.TerminalModes{
			ssh.ECHO:          1,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}

		term := os.Getenv("TERM")
		if term == "" {
			term = "xterm"
		}

		if err := session.RequestPty(term, 80, 40, modes); err != nil {
			return fmt.Errorf("request for pseudo terminal failed: %w", err)
		}

		if err := session.Shell(); err != nil {
			return fmt.Errorf("failed to start shell: %w", err)
		}
		session.Wait()
	}

	return nil
}

// ShoveBinaryFile takes a host, ssh key, user, local file path, and remote destination. It copies the
// file to the remote host, marks it executable, and returns any error.
func ShoveBinaryFile(host, privateKeyPath, user, localFilePath, destFilePath string) error {
	client, err := NewSSHClient(host, privateKeyPath, user)
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
