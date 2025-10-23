package catmsg

import (
	"encoding/binary"
	"fmt"
)

type Action interface {
	// key and value are references, not the source of data. They must be copied.
	OnPair(message Data, key []byte, value []byte) error
	// Send a database(s) back to the peer.
	OnDump() error
}

type Data interface {
	Slice() []byte
}
type Parser struct {
	version uint32
}

func NewParser() *Parser {
	p := new(Parser)
	p.version = 0
	return p
}

// Parse parses and inserts a key value pair into the database.
func (kv *KVStore) Parse(action Action, message Data) error {
	kv.checkVersion++
	data := message.Slice()
	if len(data) < 1+4+1 {
		return ErrInsufficientBytes
	}
	i := 0
	var err error
	switch data[i] {
	case CMD_PAIR:
		// The binary format is [CMD, 1B][version,4B uint32][key_size,1B uint8][key,?B][value_size,2B uint16][value,?B]
		{
			i += 1
			checkVersion := binary.LittleEndian.Uint32(data[i : i+4])
			i += 4
			if kv.checkVersion != checkVersion {
				return fmt.Errorf("version mismatch: %d vs %d", kv.checkVersion, data[i])
			}
			keySize := int(data[i])
			i += 1
			if MaxKeySize < keySize {
				return ErrKeyTooBig
			}
			if len(data)-i < keySize {
				return ErrInsufficientBytes
			}
			key := data[i : i+keySize]
			i += keySize
			if len(data)-i < 2 {
				return ErrInsufficientBytes
			}
			valueSize := int(binary.LittleEndian.Uint16(data[i : i+2]))
			i += 2
			if len(data)-i < valueSize {
				return ErrInsufficientBytes
			}
			value := data[i : i+valueSize]

			err = kv.Put(key, value, nil)
			if err != nil {
				return err
			}
			err = action.OnPair(message, key, value)
		}
	case CMD_DUMP:
		// The binary format is [CMD, 1B]
		err = action.OnDump()
	default:
		err = fmt.Errorf("unknown command %d", data[i])
	}
	return err
}
