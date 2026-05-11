package catmsg

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"time"

	sgo "git.noncepad.com/pkg/solana-go"
)

// MessageInboundCallback handles messages after they have been parsed by Parser.
type MessageInboundCallback interface {
	// OnPubkey is for sending a Public Key with a name tag.
	OnPubkey(key []byte, pubkey sgo.PublicKey) error
	// OnDump has it where a key and value are references, not the source of data. They must be copied.
	// OnMessage(message Data, key []byte, value []byte) error
	// Send a database(s) back to the peer.
	OnDump() error
	// OnPong indicates the web assembly bot has responded with a Pong to a Ping
	OnPong(time.Time) error
	// OnCustom is for custom messages.
	OnCustom(*Pair) error
}

type Data interface {
	Slice() []byte
}
type ExternalDeserializer struct {
	logger  *slog.Logger
	nonce   uint32
	version Version
	action  MessageInboundCallback
}

func NewExternalDeserializer(action MessageInboundCallback, logger *slog.Logger) *ExternalDeserializer {
	p := new(ExternalDeserializer)
	p.action = action
	p.logger = logger
	p.nonce = 0
	p.version = ProtoclV1
	return p
}

func (p *ExternalDeserializer) Version() Version {
	return p.version
}

func (p *ExternalDeserializer) Nonce() uint32 {
	return p.nonce
}

type Pair struct {
	Key   []byte
	Value []byte
}

func (p *ExternalDeserializer) Parse(message Message) error {
	if p.nonce != message.nonce {
		return fmt.Errorf("mismatch nonce: %d vs %d", p.nonce, message.nonce)
	}
	p.nonce++
	data := message.Slice()
	if len(data) < headerSize {
		return ErrInsufficientBytes
	}
	// The binary format is [CMD, 1B][version,1B][nonce,4B uint32]
	var err error
	var cmdValue uint8
	var checkVersion uint8
	var checkNonce uint32
	i := 0
	{
		target := 1
		if len(data)-i < target {
			return ErrInsufficientBytes
		}
		cmdValue = data[i]
		i += target
	}
	{
		target := 1
		if len(data)-i < target {
			return ErrInsufficientBytes
		}
		checkVersion = data[i]
		i += target
		if p.version != checkVersion {
			return fmt.Errorf("version mismatch: %d vs %d", p.version, checkVersion)
		}
	}
	{
		target := 4
		if len(data)-i < target {
			return ErrInsufficientBytes
		}
		checkNonce = binary.LittleEndian.Uint32(data[i : i+target])
		i += target
		if p.nonce != checkNonce {
			return fmt.Errorf("nonce mismatch: %d vs %d", p.nonce, checkNonce)
		}
		p.nonce++
	}
	remainingData := data[i:]
	switch cmdValue {
	case CmdDump:
		// The binary format is [CMD, 1B]
		err = p.action.OnDump()
	case CmdPong:
		if len(remainingData) != 8 {
			return ErrInsufficientBytes
		}
		v := binary.LittleEndian.Uint64(remainingData)
		if v == 0 {
			return errors.New("blank Pong timestamp")
		}
		t := time.Unix(int64(v), 0)
		p.logger.Debug(fmt.Sprintf("PONG: value %s", t))
		err = p.action.OnPong(t)
		if err != nil {
			return err
		}
	case CmdPubkey:
		var pair *Pair
		pair, err = extractPair(remainingData)
		if err != nil {
			return err
		}
		if len(pair.Value) != sgo.PublicKeyLength {
			return fmt.Errorf("bad public key length: %d", len(pair.Value))
		}
		var pubkey sgo.PublicKey
		if copy(pubkey[:], pair.Value) != sgo.PublicKeyLength {
			return fmt.Errorf("bad public key length: %d", len(pair.Value))
		}
		err = p.action.OnPubkey(pair.Key, pubkey)
		if err != nil {
			return err
		}
	case CmdCustom:
		// key value
		var pair *Pair
		pair, err = extractPair(remainingData)
		if err != nil {
			return err
		}
		err = p.action.OnCustom(pair)
		if err != nil {
			return err
		}
	default:
		err = fmt.Errorf("unknown command %d", data[i])
	}
	log.Printf("___kv nonce %d", p.nonce)
	return err
}

func extractPair(remainingData []byte) (*Pair, error) {
	// key value
	k := 0
	if len(remainingData[k:]) != 1 {
		return nil, ErrInsufficientBytes
	}
	lenK := int(remainingData[k])
	k++
	if lenK == 0 || MaxKeySize < lenK {
		return nil, errors.New("bad key size")
	}
	key := make([]byte, lenK)
	copy(key[:], remainingData[k:k+lenK])
	k += lenK
	if len(remainingData[k:]) < 2 {
		return nil, ErrInsufficientBytes
	}
	lenV := int(binary.LittleEndian.Uint16(remainingData[k : k+2]))
	if MaxValueSize < lenV {
		return nil, errors.New("bad value size")
	}
	k += 2
	if len(remainingData[k:]) < lenV {
		return nil, ErrInsufficientBytes
	}
	value := make([]byte, lenV)
	copy(value[:], remainingData[k:k+lenV])
	k += lenV
	if len(remainingData[k:]) != 0 {
		return nil, fmt.Errorf("mismatch data size: %d vs %d", 0, len(remainingData[k:]))
	}
	return &Pair{Key: key, Value: value}, nil
}
