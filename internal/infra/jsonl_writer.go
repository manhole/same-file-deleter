package infra

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type JSONLAtomicWriter struct {
	target    string
	tmpPath   string
	file      *os.File
	buffered  *bufio.Writer
	encoder   *json.Encoder
	committed bool
}

func NewJSONLAtomicWriter(target string) (*JSONLAtomicWriter, error) {
	file, tmpPath, absTarget, err := openAtomicFile(target)
	if err != nil {
		return nil, err
	}

	buffered := bufio.NewWriterSize(file, 1024*1024)
	return &JSONLAtomicWriter{
		target:   absTarget,
		tmpPath:  tmpPath,
		file:     file,
		buffered: buffered,
		encoder:  json.NewEncoder(buffered),
	}, nil
}

func (w *JSONLAtomicWriter) Write(v any) error {
	if w == nil || w.file == nil {
		return errors.New("writer is not initialized")
	}
	if w.committed {
		return errors.New("writer already committed")
	}
	if err := w.encoder.Encode(v); err != nil {
		return fmt.Errorf("encode jsonl record: %w", err)
	}
	return nil
}

func (w *JSONLAtomicWriter) Commit() error {
	if w == nil || w.file == nil {
		return errors.New("writer is not initialized")
	}
	if w.committed {
		return errors.New("writer already committed")
	}
	if err := w.buffered.Flush(); err != nil {
		return fmt.Errorf("flush jsonl output: %w", err)
	}
	if err := commitAtomicFile(w.file, w.tmpPath, w.target); err != nil {
		return err
	}
	w.committed = true
	w.file = nil
	w.tmpPath = ""
	return nil
}

func (w *JSONLAtomicWriter) Abort() {
	if w == nil || w.committed {
		return
	}
	abortAtomicFile(w.file, w.tmpPath)
	w.file = nil
	w.tmpPath = ""
}
