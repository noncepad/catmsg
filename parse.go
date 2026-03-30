package catmsg

import (
	"encoding/binary"
	"fmt"
	"log"
	"time"

	sgo "git.noncepad.com/pkg/solana-go"
)

type Action interface {
	OnWallet(key []byte, pubkey sgo.PublicKey) error
	// key and value are references, not the source of data. They must be copied.
	// OnMessage(message Data, key []byte, value []byte) error
	// Send a database(s) back to the peer.
	OnDump() error
	OnPong(time.Time) error
	OnCustom(*Pair) error
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
	data := message.Slice()
	if len(data) < 1+4+1 {
		return nil, ErrInsufficientBytes
	}
	var pair *Pair
	i := 0
	cmdValue := data[i]
	var err error
	i += 1
	checkNonce := binary.LittleEndian.Uint32(data[i : i+4])
	i += 4
	kv.nonce++
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
	pair = new(Pair)
	pair.Key = make([]byte, len(key))
	copy(pair.Key[:], key[:])
	pair.Value = make([]byte, len(value))
	copy(pair.Value[:], value[:])
	switch cmdValue {
	case CMD_PAIR:
		// The binary format is [CMD, 1B][version,4B uint32][key_size,1B uint8][key,?B][value_size,2B uint16][value,?B]
		err = action.OnMessage(message, pair.Key, pair.Value)
		if err != nil {
			return nil, err
		}
	case CMD_DUMP:
		// The binary format is [CMD, 1B]
		err = action.OnDump()
	case CMD_PONG:
		// The binary format is [CMD, 1B][version,4B uint32][key_size,1B uint8][key,?B][value_size,2B uint16][value,?B]
		err = kv.Put(key, value, nil)
		if err != nil {
			return nil, err
		}
		err = action.OnPong(time.Unix(int64(binary.LittleEndian.Uint64(value)), 0))
		if err != nil {
			return nil, err
		}
	case CMD_CUSTOM:
		err = action.OnCustom(pair)
		if err != nil {
			return nil, err
		}
	case CMD_PUBKEY:
		if len(value) != sgo.PublicKeyLength {
			return nil, fmt.Errorf("bad public key length: %d", len(value))
		}
		var pubkey sgo.PublicKey
		if copy(pubkey[:], value) != sgo.PublicKeyLength {
			return nil, fmt.Errorf("bad public key length: %d", len(value))
		}
		err = action.OnWallet(key, pubkey)
		if err != nil {
			return nil, err
		}
	default:
		err = fmt.Errorf("unknown command %d", data[i])
	}
	log.Printf("___kv nonce %d", kv.nonce)
	return pair, err
}
