package util

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var protoStateRegexp = regexp.MustCompile(`^(\w+)\[(.+?)\]$`)

type UsernetEntry struct {
	Protocol   string
	State      string
	FD         int
	SourceAddr string
	SourcePort int
	DestAddr   string
	DestPort   int
	RecvQ      int
	SendQ      int
}

// ParseUsernetInfo parses the output of 'info usernet' from QEMU's QMP
func ParseUsernetInfo(output string) ([]UsernetEntry, error) {
	var entries []UsernetEntry

	scanner := bufio.NewScanner(strings.NewReader(output))
	re := regexp.MustCompile(`^(\w+)\[(.*?)\]\s+(\d+)\s+([0-9\.]+)\s+(\d+)\s+([0-9\.]+)\s+(\d+)\s+(\d+)\s+(\d+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip Hub lines and the header line
		if strings.HasPrefix(line, "Hub") || strings.HasPrefix(line, "Protocol[State]") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		fd, _ := strconv.Atoi(matches[3])
		srcPort, _ := strconv.Atoi(matches[5])
		dstPort, _ := strconv.Atoi(matches[7])
		recvQ, _ := strconv.Atoi(matches[8])
		sendQ, _ := strconv.Atoi(matches[9])

		entry := UsernetEntry{
			Protocol:   matches[1],
			State:      matches[2],
			FD:         fd,
			SourceAddr: matches[4],
			SourcePort: srcPort,
			DestAddr:   matches[6],
			DestPort:   dstPort,
			RecvQ:      recvQ,
			SendQ:      sendQ,
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan input: %w", err)
	}

	return entries, nil
}
