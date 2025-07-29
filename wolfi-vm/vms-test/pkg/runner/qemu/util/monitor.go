package util

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

func MonitorSendReceive(conn net.Conn, cmd string) (string, error) {
	fmt.Printf("writing cmd '%s'\n", cmd)
	_, err := conn.Write([]byte(cmd + "\n"))
	if err != nil {
		return "", fmt.Errorf("failed to write command to QEMU monitor: %w", err)
	}

	var response strings.Builder
	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("got error in read: %v\n", err)
			if err == io.EOF {
				fmt.Printf("got an EOF after rading '%s'\n", line)
				break
			}
			return "", fmt.Errorf("error reading from QEMU monitor: %w", err)
		}

		response.WriteString(line)

		fmt.Printf("response: %s\n", response.String())

		if strings.HasPrefix(line, "(qemu)") {
			break
		}
	}

	return response.String(), nil
}

func MonitorConnect(ctx context.Context, socketPath string, timeout time.Duration) (net.Conn, error) {
	conn, err := PollUnixSocket(ctx, socketPath, 10*time.Second)
	if err != nil {
		return nil, err
	}
	conn.Write([]byte("\n"))

	reader := bufio.NewReader(conn)
	var response strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("got error in read: %v\n", err)
			if err == io.EOF {
				fmt.Printf("got an EOF after rading '%s'\n", line)
				break
			}
			conn.Close()
			return nil, fmt.Errorf("error reading from QEMU monitor: %w", err)
		}

		response.Write([]byte(line))

		if strings.HasPrefix(line, "(qemu)") {
			break
		}
	}

	fmt.Printf("got %s\n", response.String())

	return conn, nil
}

func MonitorInfoUsernet(conn net.Conn) ([]UsernetEntry, error) {
	response, err := MonitorSendReceive(conn, "info usernet")
	if err != nil {
		return []UsernetEntry{}, err
	}

	return ParseUsernetInfo(response)
}
