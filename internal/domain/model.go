package domain

import (
	"errors"
	"fmt"
)

type IndexRecord struct {
	Root     string `json:"root,omitempty"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	MTimeNS  int64  `json:"mtime_ns"`
	Algo     string `json:"algo"`
	Checksum string `json:"checksum"`
	Type     string `json:"type"`
}

func (r IndexRecord) Validate() error {
	if r.Path == "" {
		return errors.New("path is required")
	}
	if r.Size < 0 {
		return fmt.Errorf("size must be >= 0: %d", r.Size)
	}
	if r.Algo == "" {
		return errors.New("algo is required")
	}
	if r.Checksum == "" {
		return errors.New("checksum is required")
	}
	if r.Type == "" {
		return errors.New("type is required")
	}
	return nil
}

type PlanRecord struct {
	BRoot    string `json:"b_root"`
	Path     string `json:"path"`
	Reason   string `json:"reason"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
	KeptPath string `json:"kept_path,omitempty"` // --self モードで残すファイルの相対パス
}

func (r PlanRecord) Validate() error {
	if r.BRoot == "" {
		return errors.New("b_root is required")
	}
	if r.Path == "" {
		return errors.New("path is required")
	}
	if r.Reason == "" {
		return errors.New("reason is required")
	}
	if r.Checksum == "" {
		return errors.New("checksum is required")
	}
	if r.Size < 0 {
		return fmt.Errorf("size must be >= 0: %d", r.Size)
	}
	return nil
}
