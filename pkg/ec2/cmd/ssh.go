package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var (
	privateKeyPath string
	sshUser        string
)

func sshToInstance(host, privateKeyPath, user string, command []string) error {
	key, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("unable to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("unable to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", host), config)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Handle command vs interactive shell
	if len(command) > 0 {
		cmdString := strings.Join(command, " ")
		output, err := session.CombinedOutput(cmdString)
		if err != nil {
			return fmt.Errorf("command error: %w\nOutput: %s", err, output)
		}
		fmt.Print(string(output))
	} else {
		// Interactive shell
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

var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "SSH into an EC2 instance by tag",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
		if err != nil {
			log.Fatalf("failed to load AWS config: %v", err)
		}
		client := ec2.NewFromConfig(cfg)

		// Find instance by tag
		out, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
			Filters: []ec2types.Filter{
				{Name: aws.String("tag:Name"), Values: []string{tagName}},
				{Name: aws.String("instance-state-name"), Values: []string{"running"}},
			},
		})
		if err != nil {
			log.Fatalf("describe failed: %v", err)
		}

		var publicIP string
		for _, r := range out.Reservations {
			for _, inst := range r.Instances {
				if inst.PublicIpAddress != nil {
					publicIP = *inst.PublicIpAddress
					break
				}
			}
		}
		if publicIP == "" {
			log.Fatalf("No running instance found with tag %s", tagName)
		}

		if err := sshToInstance(publicIP, privateKeyPath, sshUser, args); err != nil {
			log.Fatalf("SSH error: %v", err)
		}
	},
}

func init() {
	sshCmd.Flags().StringVar(&tagName, "tag", "", "Tag name of the instance (required)")
	sshCmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	sshCmd.Flags().StringVar(&sshUser, "user", "ec2-user", "SSH username")
	sshCmd.Flags().StringVar(&privateKeyPath, "private-key", "id_ed25519", "Path to private SSH key")
	sshCmd.MarkFlagRequired("tag")
	sshCmd.MarkFlagRequired("region")
}

func connectSSH(host, privateKeyPath, user string) {
	key, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		log.Fatalf("Unable to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("Unable to parse private key: %v", err)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", host), config)
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput("echo 'SSH connection successful'")
	if err != nil {
		log.Fatalf("Command failed: %v", err)
	}
	log.Printf("SSH Output: %s", output)
}
