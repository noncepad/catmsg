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
	OnCustom(FixedPair) error
}

type Data interface {
	Slice() []byte
}
type ExternalDeserializer struct {
	logger  *slog.Logger
	xp      *FixedPair
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
	p.xp = new(FixedPair)
	return p
}

func (p *ExternalDeserializer) Version() Version {
	return p.version
}

func (p *ExternalDeserializer) Nonce() uint32 {
	return p.nonce
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
		var pair FixedPair
		pair, err = p.xp.extractPair(remainingData)
		if err != nil {
			return err
		}
		if pair.lenV != sgo.PublicKeyLength {
			return fmt.Errorf("bad public key length: %d", pair.lenV)
		}
		var pubkey sgo.PublicKey
		if copy(pubkey[:], pair.Value()) != sgo.PublicKeyLength {
			return fmt.Errorf("bad public key length: %d", pair.lenV)
		}
		err = p.action.OnPubkey(pair.Key(), pubkey)
		if err != nil {
			return err
		}
	case CmdCustom:
		// key value
		var pair FixedPair
		pair, err = p.xp.extractPair(remainingData)
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

func (xp *FixedPair) extractPair(remainingData []byte) (FixedPair, error) {
	// key value
	k := 0
	if len(remainingData[k:]) != 1 {
		return *xp, ErrInsufficientBytes
	}
	xp.lenK = int(remainingData[k])
	k++
	if xp.lenK == 0 || MaxKeySize < xp.lenK {
		return *xp, errors.New("bad key size")
	}
	copy(xp.key[:xp.lenK], remainingData[k:k+xp.lenK])
	k += xp.lenK
	if len(remainingData[k:]) < 2 {
		return *xp, ErrInsufficientBytes
	}
	xp.lenV = int(binary.LittleEndian.Uint16(remainingData[k : k+2]))
	if MaxValueSize < xp.lenV {
		return *xp, errors.New("bad value size")
	}
	k += 2
	if len(remainingData[k:]) < xp.lenV {
		return *xp, ErrInsufficientBytes
	}
	copy(xp.value[:xp.lenV], remainingData[k:k+xp.lenV])
	k += xp.lenV
	if len(remainingData[k:]) != 0 {
		return *xp, fmt.Errorf("mismatch data size: %d vs %d", 0, len(remainingData[k:]))
	}
	return *xp, nil
}
