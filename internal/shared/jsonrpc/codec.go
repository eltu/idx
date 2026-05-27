package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	headerContentLength = "Content-Length"
	headerCRLF          = "\r\n"
)

// ReadMessage reads one JSON-RPC 2.0 message from r using Content-Length framing.
func ReadMessage(r *bufio.Reader) (*Message, error) {
	contentLength, err := readContentLength(r)
	if err != nil {
		return nil, err
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("failed to read JSON-RPC body of %d bytes: %w", contentLength, err)
	}

	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC message: %w", err)
	}

	return &msg, nil
}

// readContentLength parses headers until the blank separator line.
func readContentLength(r *bufio.Reader) (int, error) {
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return 0, fmt.Errorf("failed to read JSON-RPC header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 && parts[0] == headerContentLength {
			contentLength, err = strconv.Atoi(parts[1])
			if err != nil {
				return 0, fmt.Errorf("invalid %s value %q: %w", headerContentLength, parts[1], err)
			}
		}
	}
	if contentLength < 0 {
		return 0, fmt.Errorf("missing %s header", headerContentLength)
	}
	return contentLength, nil
}

// WriteMessage encodes msg as JSON with Content-Length framing.
func WriteMessage(w io.Writer, msg Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to encode JSON-RPC message: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s: %d%s%s%s", headerContentLength, len(body), headerCRLF, headerCRLF, body)
	return err
}
