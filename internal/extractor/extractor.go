package extractor

import (
	"fmt"
	"io"
	"kagami_rpago/internal/rpa"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

func Extract(archive *rpa.Xp3rpaArchive, inFile string) error {
	base := filepath.Base(inFile)
	outDir := filepath.Join(filepath.Dir(inFile), strings.TrimSuffix(base, filepath.Ext(base)))
	workers := runtime.NumCPU()

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	in, err := os.Open(archive.Path)
	if err != nil {
		return err
	}
	defer in.Close()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	sem := make(chan struct{}, workers)

	for _, entry := range archive.Entries {
		sem <- struct{}{}
		wg.Add(1)
		go func(entry rpa.FileEntry) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := writeEntry(in, outDir, entry); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(entry)
	}
	wg.Wait()

	return firstErr
}

func writeEntry(in *os.File, outDir string, entry rpa.FileEntry) error {
	target := filepath.Join(outDir, entry.Name)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	out, err := os.Create(target)
	if err != nil {
		return err
	}

	writeErr := copyEntry(out, in, entry)
	closeErr := out.Close()
	fmt.Printf("Unpack %s \n", entry.Name)

	if writeErr != nil {
		return fmt.Errorf("%s: %w", entry.Name, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("%s: %w", entry.Name, closeErr)
	}
	return nil
}

func copyEntry(out *os.File, in *os.File, entry rpa.FileEntry) error {
	for _, segment := range entry.Segments {
		if len(segment.Prefix) > 0 {
			if _, err := out.Write(segment.Prefix); err != nil {
				return err
			}
		}

		reader := io.NewSectionReader(in, segment.Offset, segment.Length)
		if _, err := io.Copy(out, reader); err != nil {
			return err
		}
	}
	return nil
}
