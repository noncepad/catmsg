package catmsg

import (
	"errors"
	"fmt"
)

const (
	BUFMAX int = 4 * 1024
)

type FixedPair struct {
	lenK  int
	key   [MaxKeySize]byte
	lenV  int
	value [MaxValueSize]byte
}

func (fp *FixedPair) From(key []byte, value []byte) error {
	if len(key) == 0 {
		return errors.New("blank key")
	}
	if MaxKeySize < len(key) {
		return fmt.Errorf("key to big: %d %d", MaxKeySize, len(key))
	}
	if MaxValueSize < len(value) {
		return fmt.Errorf("key to big: %d %d", MaxValueSize, len(value))
	}
	fp.lenK = len(key)
	copy(fp.key[0:fp.lenK], key[:])
	fp.lenV = len(value)
	copy(fp.value[0:fp.lenV], value[:])
	return nil
}

func (fp *FixedPair) Key() []byte {
	return fp.key[:fp.lenK]
}

func (fp *FixedPair) Value() []byte {
	return fp.value[:fp.lenV]
}

type Version = uint8

const (
	ProtoclV1 Version = 1
)

type CommandTag = uint8

const (
	// internal -> external
	CmdDump   CommandTag = 1
	CmdPong   CommandTag = 2
	CmdCustom CommandTag = 3
	CmdPubkey CommandTag = 4
	// external -> internal
	CmdPing     CommandTag = 5
	CmdShutdown CommandTag = 6
)
