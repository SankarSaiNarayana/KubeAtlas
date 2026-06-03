package ingest

import (
	"bufio"
	"context"
	"os"
	"time"
)

func tailAuditFile(ctx context.Context, path string, handle func([]byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(0, os.SEEK_END); err != nil {
		return err
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if scanner.Scan() {
			_ = handle(scanner.Bytes())
			continue
		}
		if err := scanner.Err(); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}
