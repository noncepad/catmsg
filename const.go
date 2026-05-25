package catmsg

// IsArrayEqual checks if two arrays are equal.
func IsArrayEqual[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

const (
	MaxKeySize   = 64
	MaxValueSize = BUFMAX - MaxKeySize - 128
	EntrySize    = MaxKeySize + 4 + MaxValueSize // 1092 bytes
)

const (
	EnvMothershipWallet string = "MOTHERSHIP_PUBKEY"
	EnvRuntimePipeline  string = "PIPELINE"
	EnvLogLevel         string = "LOG_LEVEL"
)
