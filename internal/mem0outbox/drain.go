package mem0outbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// DrainReport summarises one QuotaDrainer.Drain call.
type DrainReport struct {
	Drained  int
	Skipped  int
	Pending  int
	Duration time.Duration
}

// QuotaDrainer drains buffered outbox writes to both managed and OSS
// backends exactly once. It uses a separate cursor file from the
// regular Flusher so the two can coexist without collision.
//
// Idempotency: the cursor advances only after both Managed and OSS
// pushes succeed. A retry after a crash picks up at the exact byte
// offset that failed, so no capsule is double-pushed.
type QuotaDrainer struct {
	PendingPath string
	CursorPath  string
	Managed     Mem0Pusher
	OSS         Mem0Pusher
	BatchSize   int
	DryRun      bool
}

// Drain reads pending capsules from the cursor offset and pushes each
// to both Managed and OSS. In --dry-run mode it counts pending items
// without calling either backend.
func (d *QuotaDrainer) Drain(ctx context.Context) (DrainReport, error) {
	var report DrainReport
	start := time.Now()
	defer func() { report.Duration = time.Since(start) }()

	if d.DryRun {
		return d.dryRun()
	}

	if d.Managed == nil || d.OSS == nil {
		return report, errors.New("drain: both managed and oss pushers required")
	}

	limit := d.BatchSize
	if limit <= 0 {
		limit = 100
	}

	in, err := os.Open(d.PendingPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return report, nil
		}
		return report, fmt.Errorf("open pending: %w", err)
	}
	defer in.Close()

	cursor, err := readCursor(d.CursorPath)
	if err != nil {
		return report, fmt.Errorf("read cursor: %w", err)
	}
	if cursor > 0 {
		if _, err := in.Seek(cursor, io.SeekStart); err != nil {
			return report, fmt.Errorf("seek to cursor: %w", err)
		}
	}

	br := bufio.NewReader(in)
	offset := cursor
	processed := 0

	for processed < limit {
		line, readErr := br.ReadBytes('\n')
		if len(line) == 0 && errors.Is(readErr, io.EOF) {
			break
		}

		lineLen := int64(len(line))
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			offset += lineLen
			if errors.Is(readErr, io.EOF) {
				break
			}
			continue
		}

		var c Capsule
		if jerr := json.Unmarshal(trimmed, &c); jerr != nil {
			report.Skipped++
			offset += lineLen
			if errors.Is(readErr, io.EOF) {
				break
			}
			continue
		}

		if err := d.Managed.Push(ctx, c); err != nil {
			return report, fmt.Errorf("managed push (offset=%d): %w", offset, err)
		}
		if err := d.OSS.Push(ctx, c); err != nil {
			return report, fmt.Errorf("oss push (offset=%d): %w", offset, err)
		}

		offset += lineLen
		report.Drained++
		processed++

		if writeErr := writeCursor(d.CursorPath, offset); writeErr != nil {
			return report, fmt.Errorf("write cursor: %w", writeErr)
		}

		if errors.Is(readErr, io.EOF) {
			break
		}
	}

	return report, nil
}

func (d *QuotaDrainer) dryRun() (DrainReport, error) {
	var report DrainReport

	in, err := os.Open(d.PendingPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return report, nil
		}
		return report, fmt.Errorf("open pending: %w", err)
	}
	defer in.Close()

	cursor, err := readCursor(d.CursorPath)
	if err != nil {
		return report, fmt.Errorf("read cursor: %w", err)
	}
	if cursor > 0 {
		if _, err := in.Seek(cursor, io.SeekStart); err != nil {
			return report, fmt.Errorf("seek: %w", err)
		}
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var c Capsule
		if err := json.Unmarshal(line, &c); err != nil {
			report.Skipped++
			continue
		}
		report.Pending++
	}
	return report, scanner.Err()
}
