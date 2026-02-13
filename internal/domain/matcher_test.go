package domain

import "testing"

func TestMatchKeyFromIndex(t *testing.T) {
	rec := IndexRecord{
		Algo:     AlgoBLAKE3,
		Checksum: "abc123",
		Size:     42,
	}

	key := MatchKeyFromIndex(rec)
	if key.Algo != AlgoBLAKE3 {
		t.Fatalf("unexpected algo: %s", key.Algo)
	}
	if key.Checksum != "abc123" {
		t.Fatalf("unexpected checksum: %s", key.Checksum)
	}
	if key.Size != 42 {
		t.Fatalf("unexpected size: %d", key.Size)
	}
}
