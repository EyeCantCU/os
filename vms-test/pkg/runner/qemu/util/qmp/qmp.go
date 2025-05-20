package qmp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/qemu/util"
)

// Command represents a QMP command to be sent to the QEMU monitor.
type Command struct {
	Execute   string      `json:"execute"`
	Arguments interface{} `json:"arguments,omitempty"`
}

// Response represents a typical QMP response from the monitor.
type Response struct {
	Return json.RawMessage `json:"return,omitempty"`
	Error  *QMPError       `json:"error,omitempty"`
}

// QMPError represents an error returned from QMP.
type QMPError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

// VersionInfo represents QEMU version info in the greeting.
type VersionInfo struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Micro int `json:"micro"`
}

// QMPGreeting holds the QMP section of the greeting.
type QMPGreeting struct {
	Version struct {
		QEMU    VersionInfo `json:"qemu"`
		Package string      `json:"package"`
	} `json:"version"`
	Capabilities []string `json:"capabilities"`
}

// Greeting represents the full QMP greeting message.
type Greeting struct {
	QMP QMPGreeting `json:"QMP"`
}

// QMP represents a connection to the QEMU Monitor Protocol.
type QMP struct {
	Path     string
	conn     net.Conn
	reader   *bufio.Reader
	mu       sync.Mutex
	Greeting *Greeting
}

// get versio info
func (q *QMP) Version() VersionInfo {
	if q.Greeting == nil {
		return VersionInfo{}
	}
	return q.Greeting.QMP.Version.QEMU
}

func (q *QMP) Capabilities() []string {
	if q.Greeting == nil {
		return []string{}
	}
	return q.Greeting.QMP.Capabilities
}

// Connect establishes a Unix socket connection to the QEMU monitor
// and parses the initial greeting message.
func (q *QMP) Connect(ctx context.Context, timeout time.Duration) error {

	conn, err := util.PollUnixSocket(ctx, q.Path, timeout)
	if err != nil {
		return err
	}
	q.conn = conn
	q.reader = bufio.NewReader(conn)

	// Read and parse the QMP greeting
	line, err := q.reader.ReadBytes('\n')
	if err != nil {
		q.conn.Close()
		return err
	}

	var greeting Greeting
	if err := json.Unmarshal(line, &greeting); err != nil {
		q.conn.Close()
		return err
	}
	q.Greeting = &greeting

	// must be sent after connecting
	_, err = q.Send(Command{Execute: "qmp_capabilities"})

	return err
}

// Close shuts down the connection to the QEMU monitor.
func (q *QMP) Close() error {
	if q.conn == nil {
		return errors.New("connection not established")
	}
	return q.conn.Close()
}

// Send sends a QMP command and returns the response.
func (q *QMP) Send(cmd Command) (*Response, error) {
	if q.conn == nil {
		return nil, errors.New("connection not established")
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	data = append(data, '\n')
	_, err = q.conn.Write(data)
	if err != nil {
		return nil, err
	}

	line, err := q.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal error, content: %s: %v", line, err)
	}

	if resp.Error != nil {
		return &resp, errors.New(resp.Error.Desc)
	}

	return &resp, nil
}

func (q *QMP) HumanMonitorCommand(cmdline string) (string, error) {
	var rstr string

	cmd := Command{Execute: "human-monitor-command",
		Arguments: map[string]string{"command-line": cmdline}}

	result, err := q.Send(cmd)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(result.Return, &rstr); err != nil {
		return "", fmt.Errorf("Unable to unmarshal response from %s to string: %v", cmd.Execute, err)
	}

	return rstr, nil
}
