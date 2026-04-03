package catmsg

import (
	"encoding/binary"
	"errors"
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
	// The binary format is [CMD, 1B][version,4B uint32][key_size,1B uint8][key,?B][value_size,2B uint16][value,?B]
	var err error
	var cmdValue uint8
	var checkNonce uint32
	pair := new(Pair)
	i := 0
	{
		target := 1
		if len(data)-i < target {
			return nil, ErrInsufficientBytes
		}
		cmdValue = data[i]
		i += target
	}
	{
		target := 4
		if len(data)-i < target {
			return nil, ErrInsufficientBytes
		}
		checkNonce = binary.LittleEndian.Uint32(data[i : i+target])
		i += target
		if kv.nonce != checkNonce {
			// log.Printf("version mismatch: %d vs %d", kv.nonce, checkNonce)
			return nil, fmt.Errorf("version mismatch: %d vs %d", kv.nonce, checkNonce)
		}
		kv.nonce++
	}
	{
		target := 1
		keySize := int(data[i])
		i += target
		if MaxKeySize < keySize {
			return nil, ErrKeyTooBig
		}
		target = keySize
		if len(data)-i < target {
			return nil, ErrInsufficientBytes
		}
		pair.Key = make([]byte, target)
		copy(pair.Key, data[i:i+target])
		i += target
	}
	{
		target := 2
		if len(data)-i < target {
			return nil, ErrInsufficientBytes
		}
		valueSize := int(binary.LittleEndian.Uint16(data[i : i+target]))
		i += target
		target = valueSize
		if len(data)-i < target {
			return nil, ErrInsufficientBytes
		}
		pair.Value = make([]byte, target)
		copy(pair.Value, data[i:i+target])
		i += target
		if i < len(data) {
			return nil, fmt.Errorf("failed to read all data: %d %d", i, len(data))
		}
	}
	err = kv.Put(pair.Key, pair.Value, nil)
	if err != nil {
		return nil, err
	}
	switch cmdValue {
	case CMD_PAIR:
		err = errors.New("PAIR not supported")
		return nil, err
	case CMD_DUMP:
		// The binary format is [CMD, 1B]
		err = action.OnDump()
	case CMD_PONG:
		// The binary format is [CMD, 1B][version,4B uint32][key_size,1B uint8][key,?B][value_size,2B uint16][value,?B]
		v := binary.LittleEndian.Uint64(pair.Value)
		log.Printf("PONG: value %+v; v %d", pair.Value, v)
		if v == 0 {
			return nil, errors.New("blank Pong timestamp")
		}

		err = action.OnPong(time.Unix(int64(v), 0))
		if err != nil {
			return nil, err
		}
	case CMD_CUSTOM:
		err = action.OnCustom(pair)
		if err != nil {
			return nil, err
		}
	case CMD_PUBKEY:
		if len(pair.Value) != sgo.PublicKeyLength {
			return nil, fmt.Errorf("bad public key length: %d", len(pair.Value))
		}
		var pubkey sgo.PublicKey
		if copy(pubkey[:], pair.Value) != sgo.PublicKeyLength {
			return nil, fmt.Errorf("bad public key length: %d", len(pair.Value))
		}
		err = action.OnWallet(pair.Key, pubkey)
		if err != nil {
			return nil, err
		}
	default:
		err = fmt.Errorf("unknown command %d", data[i])
	}
	log.Printf("___kv nonce %d", kv.nonce)
	return pair, err
}
