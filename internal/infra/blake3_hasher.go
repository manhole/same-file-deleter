package infra

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"lukechampine.com/blake3"
)

type FileHasher interface {
	HashFile(path string) (string, error)
}

type Blake3Hasher struct{}

func (Blake3Hasher) HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := blake3.New(32, nil)
	buf := make([]byte, 1024*1024)
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			if _, err := h.Write(buf[:n]); err != nil {
				return "", fmt.Errorf("hash write: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("read file: %w", readErr)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
