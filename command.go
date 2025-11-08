package catmsg

type SettingCommand = uint8

const (
	CMD_SHUTDOWN SettingCommand = 1
)

type commandInput interface {
	Put(key []byte, value []byte, msg targetSlice) error
}

// Instruct the bot to shutdown and clean out funds back to the mothership.
func CommandShutdown(input commandInput, msg targetSlice) error {
	return input.Put([]byte{CMD_SHUTDOWN}, []byte{0}, msg)
}
