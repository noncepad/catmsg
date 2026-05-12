package catmsg

type Database struct{}

const (
	MaxKeySize   = 64
	MaxValueSize = BUFMAX - MaxKeySize - 128
	EntrySize    = MaxKeySize + 4 + MaxValueSize // 1092 bytes
)

// simpleHash is a small FNV-1a variant
func simpleHash(data []byte) uint32 {
	var h uint32 = 2166136261
	for _, b := range data {
		h ^= uint32(b)
		h *= 16777619
	}
	return h
}

func pad(b []byte, n int) []byte {
	p := make([]byte, n)
	copy(p, b)
	return p
}
