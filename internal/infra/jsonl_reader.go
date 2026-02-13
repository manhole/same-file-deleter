package infra

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"same-file-deleter/internal/domain"
)

type JSONLParseError struct {
	Path string
	Line int
	Err  error
}

func (e *JSONLParseError) Error() string {
	return fmt.Sprintf("%s:%d: %v", e.Path, e.Line, e.Err)
}

func (e *JSONLParseError) Unwrap() error {
	return e.Err
}

func ReadIndexJSONL(filePath string, onRecord func(domain.IndexRecord) error) error {
	return readJSONLLines(filePath, func(line int, raw []byte) error {
		var rec domain.IndexRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			return &JSONLParseError{Path: filePath, Line: line, Err: fmt.Errorf("unmarshal index record: %w", err)}
		}
		if err := rec.Validate(); err != nil {
			return &JSONLParseError{Path: filePath, Line: line, Err: fmt.Errorf("invalid index record: %w", err)}
		}
		if onRecord != nil {
			return onRecord(rec)
		}
		return nil
	})
}

func ReadPlanJSONL(filePath string, onRecord func(domain.PlanRecord) error) error {
	return readJSONLLines(filePath, func(line int, raw []byte) error {
		var rec domain.PlanRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			return &JSONLParseError{Path: filePath, Line: line, Err: fmt.Errorf("unmarshal plan record: %w", err)}
		}
		if err := rec.Validate(); err != nil {
			return &JSONLParseError{Path: filePath, Line: line, Err: fmt.Errorf("invalid plan record: %w", err)}
		}
		if onRecord != nil {
			return onRecord(rec)
		}
		return nil
	})
}

func readJSONLLines(filePath string, onLine func(int, []byte) error) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	line := 0
	for scanner.Scan() {
		line++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		if err := onLine(line, raw); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan jsonl: %w", err)
	}

	return nil
}
