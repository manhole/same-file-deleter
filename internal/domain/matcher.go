package domain

type MatchKey struct {
	Algo     string
	Checksum string
	Size     int64
}

func MatchKeyFromIndex(r IndexRecord) MatchKey {
	return MatchKey{
		Algo:     r.Algo,
		Checksum: r.Checksum,
		Size:     r.Size,
	}
}
