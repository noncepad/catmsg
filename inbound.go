package catmsg

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"time"

	sgo "github.com/gagliardetto/solana-go"
)

// MessageInboundCallback handles messages after they have been parsed by Parser.
type MessageInboundCallback interface {
	// OnPubkey is for sending a Public Key with a name tag.
	OnHandshake(pubkey sgo.PublicKey) error
	// OnDump has it where a key and value are references, not the source of data. They must be copied.
	// OnMessage(message Data, key []byte, value []byte) error
	// Send a database(s) back to the peer.
	OnDump() error
	// OnPong indicates the web assembly bot has responded with a Pong to a Ping
	OnPong(time.Time) error
	OnPubkey(name []byte, pubkey sgo.PublicKey) error
	// OnCustom is for custom messages.
	OnCustomRaw([]byte) error
}

type Data interface {
	Slice() []byte
}
type ExternalDeserializer struct {
	ctx            context.Context
	logger         *slog.Logger
	xp             *FixedPair
	nonce          uint32
	version        Version
	action         MessageInboundCallback
	index          int
	leftoverBuffer []byte
	hasDataChannel bool
	dataC          chan<- []byte
}

func NewExternalDeserializer(ctx context.Context, action MessageInboundCallback, logger *slog.Logger) *ExternalDeserializer {
	p := new(ExternalDeserializer)
	p.ctx = ctx
	p.action = action
	p.logger = logger
	p.nonce = 0
	p.version = ProtoclV1
	p.index = 0
	p.hasDataChannel = false
	p.leftoverBuffer = make([]byte, MaxValueSize*2)
	p.xp = new(FixedPair)
	return p
}

func (p *ExternalDeserializer) OnHandshake(dataC chan<- []byte) error {
	if p.hasDataChannel {
		return errors.New("duplicate handshake")
	}
	p.hasDataChannel = true
	p.dataC = dataC
	return nil
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
			key := pair.Key()
			if len(key) == 0 {
				return errors.New("blank key")
			}
			var pubkey sgo.PublicKey
			value := pair.Value()
			if len(value) != sgo.PublicKeyLength {
				return fmt.Errorf("public key mismatch: %d vs %d", len(value), sgo.PublicKeyLength)
			}
			copy(pubkey[:], value)
			if key[0] == 0 {
				// handshake
				err = p.action.OnHandshake(pubkey)
			} else {
				err = p.action.OnPubkey(pair.Key(), pubkey)
			}
		case CmdCustom:
			if !p.hasDataChannel {
				return errors.New("missing data channel")
			}
			var out []byte
			out, consumed, err = extractCustom(payload)
			if err == ErrInsufficientBytes {
				i = msgStart
				break loop
			} else if err != nil {
				return err
			}
			x := make([]byte, len(out))
			copy(x[:], out[:])
			select {
			case <-p.ctx.Done():
				return p.ctx.Err()
			case p.dataC <- x:
			}
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

func extractCustom(data []byte) ([]byte, int, error) {
	if len(data) < 2 {
		return nil, 0, ErrInsufficientBytes
	}
	n := int(binary.LittleEndian.Uint16(data[0:2]))
	if BUFMAX < n {
		return nil, 0, fmt.Errorf("too big: %d", n)
	}
	if len(data) < 2+n {
		return nil, 0, ErrInsufficientBytes
	}
	return data[2 : 2+n], 2 + n, nil
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
