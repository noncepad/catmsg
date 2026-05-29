package catmsg

import (
	"encoding/binary"
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

func (fp *FixedPair) Serialize(input []byte) int {
	i := 0
	input[i] = uint8(fp.lenK)
	i++
	copy(input[i:i+fp.lenK], fp.key[0:fp.lenK])
	i += fp.lenK
	{
		binary.LittleEndian.PutUint16(input[i:i+2], uint16(fp.lenV))
		i += 2
	}
	copy(input[i:i+fp.lenV], fp.value[0:fp.lenV])
	i += fp.lenV
	return i
}

func (fp *FixedPair) Deserialize(data []byte) (int, error) {
	i := 0
	fp.lenK = int(data[i])
	i += 1
	if MaxKeySize < fp.lenK {
		return 0, fmt.Errorf("bad key: %d vs %d", MaxKeySize, fp.lenK)
	}
	if len(data) < i+fp.lenK {
		return 0, fmt.Errorf("bad key data: %d %d %d", len(data), fp.lenK, i)
	}
	copy(fp.key[0:fp.lenK], data[i:i+fp.lenK])
	i += fp.lenK
	{
		fp.lenV = int(binary.LittleEndian.Uint16(data[i : i+2]))
		i += 2
		if MaxValueSize < fp.lenV {
			return 0, fmt.Errorf("bad value: %d vs %d", MaxValueSize, fp.lenV)
		}
		if len(data) < i+fp.lenV {
			return 0, fmt.Errorf("bad value data: %d %d %d", len(data), fp.lenV, i)
		}
	}
	copy(fp.value[0:fp.lenV], data[i:i+fp.lenV])
	i += fp.lenV
	return i, nil
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
	ProtoclV1 Version = 34
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
