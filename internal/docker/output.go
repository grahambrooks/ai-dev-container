package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ANSI escape codes for terminal formatting.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

// dockerStreamMessage represents a JSON message from the Docker build/pull stream.
type dockerStreamMessage struct {
	Stream      string          `json:"stream"`
	Status      string          `json:"status"`
	Progress    string          `json:"progress"`
	ID          string          `json:"id"`
	Error       string          `json:"error"`
	ErrorDetail json.RawMessage `json:"errorDetail"`
}

// streamBuildOutput reads Docker build/pull JSON stream and writes the
// extracted text to w, with ANSI color for errors and key status lines.
// Returns an error if the stream contains a build error message.
func streamBuildOutput(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	var buildErr string
	for scanner.Scan() {
		var msg dockerStreamMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			fmt.Fprintln(w, scanner.Text())
			continue
		}

		if msg.Error != "" {
			fmt.Fprintf(w, "%s%serror:%s %s\n", ansiBold, ansiRed, ansiReset, msg.Error)
			buildErr = msg.Error
			continue
		}

		if msg.Stream != "" {
			line := msg.Stream
			switch {
			case strings.HasPrefix(line, "Step "):
				fmt.Fprintf(w, "%s%s%s", ansiCyan, line, ansiReset)
			case strings.Contains(line, "Successfully built"),
				strings.Contains(line, "Successfully tagged"):
				fmt.Fprintf(w, "%s%s%s", ansiGreen, line, ansiReset)
			default:
				fmt.Fprint(w, line)
			}
		}

		if msg.Status != "" {
			formatPullStatus(w, &msg)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if buildErr != "" {
		return fmt.Errorf("build failed: %s", buildErr)
	}
	return nil
}

// formatPullStatus formats Docker pull progress messages.
func formatPullStatus(w io.Writer, msg *dockerStreamMessage) {
	if msg.ID != "" {
		if msg.Progress != "" {
			fmt.Fprintf(w, "%s: %s %s\r", msg.ID, msg.Status, msg.Progress)
		} else {
			fmt.Fprintf(w, "%s: %s\n", msg.ID, msg.Status)
		}
	} else {
		status := msg.Status
		if strings.HasPrefix(status, "Digest:") || strings.HasPrefix(status, "Status:") {
			fmt.Fprintf(w, "%s%s%s\n", ansiGreen, status, ansiReset)
		} else {
			fmt.Fprintf(w, "%s\n", status)
		}
	}
}
