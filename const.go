package catmsg

const (
	STATUSKEY_INSTRUCTION  string = "bot_instruction_v1"
	STATUSKEY_BOT_PUBKEY   string = "bot_pubkey_v1"
	STATUSKEY_BOT_LAMPORTS string = "bot_lamports_v1"
	// send a command to the bot
	SETTINGS_COMMAND string = "mothership_cmd_v1"
	SETTINGS_PING    string = "mothership_ping_v1"
)

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

func IsBotPubkey(key []byte) bool {
	x := []byte(STATUSKEY_BOT_PUBKEY)
	return IsArrayEqual(x, key)
}

func IsBotLamports(key []byte) bool {
	x := []byte(STATUSKEY_BOT_LAMPORTS)
	return IsArrayEqual(x, key)
}
