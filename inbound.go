package catmsg

import (
	"encoding/binary"
	"errors"
	"fmt"
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
	logger         *slog.Logger
	xp             *FixedPair
	nonce          uint32
	version        Version
	action         MessageInboundCallback
	index          int
	leftoverBuffer []byte
}

func NewExternalDeserializer(action MessageInboundCallback, logger *slog.Logger) *ExternalDeserializer {
	p := new(ExternalDeserializer)
	p.action = action
	p.logger = logger
	p.nonce = 0
	p.version = ProtoclV1
	p.index = 0
	p.leftoverBuffer = make([]byte, MaxValueSize*2)
	p.xp = new(FixedPair)
	return p
}

func (p *ExternalDeserializer) Version() Version {
	return p.version
}

func (p *ExternalDeserializer) Nonce() uint32 {
	return p.nonce
}

func (p *ExternalDeserializer) Parse(indata []byte) error {
	var data []byte
	if p.index > 0 {
		combined := make([]byte, p.index+len(indata))
		copy(combined, p.leftoverBuffer[:p.index])
		copy(combined[p.index:], indata)
		data = combined
		p.index = 0
	} else {
		data = indata
	}

	i := 0
loop:
	for i < len(data) {
		msgStart := i

		if len(data)-i < headerSize {
			i = msgStart
			break
		}

		cmdValue := data[i]
		i++
		checkVersion := data[i]
		i++
		if p.version != checkVersion {
			return fmt.Errorf("version mismatch: %d vs %d", p.version, checkVersion)
		}
		checkNonce := binary.LittleEndian.Uint32(data[i : i+4])
		i += 4
		if p.nonce != checkNonce {
			return fmt.Errorf("nonce mismatch: %d vs %d", p.nonce, checkNonce)
		}

		payload := data[i:]
		var consumed int
		var err error

		switch cmdValue {
		case CmdDump:
			consumed = 0
			err = p.action.OnDump()
		case CmdPong:
			if len(payload) < 8 {
				i = msgStart
				break loop
			}
			v := binary.LittleEndian.Uint64(payload[:8])
			if v == 0 {
				return errors.New("blank Pong timestamp")
			}
			err = p.action.OnPong(time.Unix(int64(v), 0))
			consumed = 8
		case CmdPubkey:
			var pair FixedPair
			pair, consumed, err = extractPairV2(payload)
			if err == ErrInsufficientBytes {
				i = msgStart
				break loop
			}
			if err != nil {
				return err
			}
			if pair.lenV != sgo.PublicKeyLength {
				return fmt.Errorf("bad public key length: %d", pair.lenV)
			}
			var pubkey sgo.PublicKey
			copy(pubkey[:], pair.Value())
			err = p.action.OnPubkey(pair.Key(), pubkey)
		case CmdCustom:
			var pair FixedPair
			pair, consumed, err = extractPairV2(payload)
			if err == ErrInsufficientBytes {
				i = msgStart
				break loop
			}
			if err != nil {
				return err
			}
			err = p.action.OnCustom(pair)
		default:
			return fmt.Errorf("unknown command %d", cmdValue)
		}

		if err != nil {
			return err
		}
		i += consumed
		p.nonce++
	}

	leftover := len(data) - i
	if leftover > 0 {
		if leftover > len(p.leftoverBuffer) {
			p.leftoverBuffer = make([]byte, leftover)
		}
		copy(p.leftoverBuffer, data[i:])
		p.index = leftover
	} else {
		p.index = 0
	}
	return nil
}

func extractPairV2(data []byte) (FixedPair, int, error) {
	var fp FixedPair
	k := 0
	if len(data)-k < 1 {
		return fp, 0, ErrInsufficientBytes
	}
	fp.lenK = int(data[k])
	k++
	if fp.lenK == 0 || MaxKeySize < fp.lenK {
		return fp, 0, errors.New("bad key size")
	}
	if len(data)-k < fp.lenK {
		return fp, 0, ErrInsufficientBytes
	}
	copy(fp.key[:fp.lenK], data[k:k+fp.lenK])
	k += fp.lenK
	if len(data)-k < 2 {
		return fp, 0, ErrInsufficientBytes
	}
	fp.lenV = int(binary.LittleEndian.Uint16(data[k : k+2]))
	if MaxValueSize < fp.lenV {
		return fp, 0, errors.New("bad value size")
	}
	k += 2
	if len(data)-k < fp.lenV {
		return fp, 0, ErrInsufficientBytes
	}
	copy(fp.value[:fp.lenV], data[k:k+fp.lenV])
	k += fp.lenV
	return fp, k, nil
}

func (fp *FixedPair) extractPair(remainingData []byte) (FixedPair, error) {
	// key value
	k := 0
	if len(remainingData[k:]) != 1 {
		return *fp, ErrInsufficientBytes
	}
	fp.lenK = int(remainingData[k])
	k++
	if fp.lenK == 0 || MaxKeySize < fp.lenK {
		return *fp, errors.New("bad key size")
	}
	copy(fp.key[:fp.lenK], remainingData[k:k+fp.lenK])
	k += fp.lenK
	if len(remainingData[k:]) < 2 {
		return *fp, ErrInsufficientBytes
	}
	fp.lenV = int(binary.LittleEndian.Uint16(remainingData[k : k+2]))
	if MaxValueSize < fp.lenV {
		return *fp, errors.New("bad value size")
	}
	k += 2
	if len(remainingData[k:]) < fp.lenV {
		return *fp, ErrInsufficientBytes
	}
	copy(fp.value[:fp.lenV], remainingData[k:k+fp.lenV])
	k += fp.lenV
	if len(remainingData[k:]) != 0 {
		return *fp, fmt.Errorf("mismatch data size: %d vs %d", 0, len(remainingData[k:]))
	}
	return *fp, nil
}
