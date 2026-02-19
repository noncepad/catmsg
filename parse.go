package catmsg

import (
	"encoding/binary"
	"fmt"
)

type Action interface {
	// key and value are references, not the source of data. They must be copied.
	OnMessage(message Data, key []byte, value []byte) error
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

type Pair struct {
	Key   []byte
	Value []byte
}

// Parse parses and inserts a key value pair into the database.
func (kv *KVStore) Parse(action Action, message Data) (*Pair, error) {
	kv.nonce++
	data := message.Slice()
	if len(data) < 1+4+1 {
		return nil, ErrInsufficientBytes
	}
	var pair *Pair
	i := 0
	var err error
	switch data[i] {
	case CMD_PAIR:
		// The binary format is [CMD, 1B][version,4B uint32][key_size,1B uint8][key,?B][value_size,2B uint16][value,?B]
		{
			i += 1
			checkNonce := binary.LittleEndian.Uint32(data[i : i+4])
			i += 4
			if kv.nonce != checkNonce {
				return nil, fmt.Errorf("version mismatch: %d vs %d", kv.nonce, checkNonce)
			}
			keySize := int(data[i])
			i += 1
			if MaxKeySize < keySize {
				return nil, ErrKeyTooBig
			}
			if len(data)-i < keySize {
				return nil, ErrInsufficientBytes
			}
			key := data[i : i+keySize]
			i += keySize
			if len(data)-i < 2 {
				return nil, ErrInsufficientBytes
			}
			valueSize := int(binary.LittleEndian.Uint16(data[i : i+2]))
			i += 2
			if len(data)-i < valueSize {
				return nil, ErrInsufficientBytes
			}
			value := data[i : i+valueSize]

			err = kv.Put(key, value, nil)
			if err != nil {
				return nil, err
			}
			err = action.OnMessage(message, key, value)
			if err != nil {
				return nil, err
			}
			pair = new(Pair)
			pair.Key = make([]byte, len(key))
			copy(pair.Key[:], key[:])
			pair.Value = make([]byte, len(value))
			copy(pair.Value[:], value[:])

		}
	case CMD_DUMP:
		// The binary format is [CMD, 1B]
		err = action.OnDump()
	default:
		err = fmt.Errorf("unknown command %d", data[i])
	}
	return pair, err
}
