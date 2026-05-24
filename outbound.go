package catmsg

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"time"

	sgo "github.com/gagliardetto/solana-go"
)

type ExternalSerializer struct {
	logger     *slog.Logger
	nonce      uint32
	version    Version
	blankKey   []byte
	byteUint64 []byte
	index      int
	buffer     []byte
	localKey   sgo.PrivateKey
}

func NewExternalSerializer(logger *slog.Logger, key sgo.PrivateKey) *ExternalSerializer {
	s := new(ExternalSerializer)
	s.logger = logger
	s.nonce = 0
	s.version = ProtoclV1
	s.blankKey = make([]byte, 0)
	s.byteUint64 = make([]byte, 8)
	s.index = 0
	s.buffer = make([]byte, BUFMAX)
	s.localKey = key
	return s
}

func (p *ExternalSerializer) OnHandshake(pubkey sgo.PublicKey) error {
	return nil
}

func (p *ExternalSerializer) Version() Version {
	return p.version
}

func (p *ExternalSerializer) Nonce() uint32 {
	return p.nonce
}

// body Pair: 1 + len(key) + 2 + len(value)

const headerSize = 1 + 1 + 4

// The binary format is [CMD, 1B][version,1B][nonce, 4B uint32][some fixed size message that can be parsed by cmd tag]
func (p *ExternalSerializer) writeHeader(cmdTag CommandTag, bodySize int) {
	if len(p.buffer) < p.index+headerSize+bodySize {
		buffer := make([]byte, len(p.buffer)+p.index+headerSize+bodySize+BUFMAX)
		copy(buffer[0:p.index], p.buffer[0:p.index])
		p.buffer = buffer
	}
	p.buffer[p.index] = cmdTag
	p.index += 1
	p.buffer[p.index] = p.version
	p.index += 1
	binary.LittleEndian.PutUint32(p.buffer[p.index:(p.index+4)], p.nonce)
	p.nonce++
	p.index += 4
}

func (p *ExternalSerializer) Ping(t time.Time) {
	p.writeHeader(CmdPing, 8)
	slice := p.buffer[p.index:(p.index + 8)]
	p.index += 8
	binary.LittleEndian.PutUint64(slice, uint64(t.Unix()))
}

func (p *ExternalSerializer) Shutdown() {
	p.writeHeader(CmdShutdown, 0)
}

type customMessage interface {
	Key() []byte
	Value() []byte
}

func (p *ExternalSerializer) Custom(custom customMessage) error {
	key := custom.Key()
	value := custom.Value()
	if key == nil {
		return errors.New("blank key")
	}
	if value == nil {
		return errors.New("blank value")
	}
	if MaxKeySize < len(key) {
		return fmt.Errorf("key too big: %d vs %d", MaxKeySize, len(key))
	}
	if MaxValueSize < len(value) {
		return fmt.Errorf("value too big: %d vs %d", MaxValueSize, len(value))
	}
	p.writeHeader(CmdCustom, 1+len(key)+2+len(value))
	p.buffer[p.index] = uint8(len(key))
	p.index += 1
	copy(p.buffer[p.index:p.index+len(key)], key[:])
	p.index += len(key)
	binary.LittleEndian.PutUint16(p.buffer[p.index:p.index+2], uint16(len(value)))
	p.index += 2
	copy(p.buffer[p.index:p.index+len(value)], value[:])
	p.index += len(value)
	return nil
}

// Flush dumps a byte slice of written data. the index resets to 0.
func (p *ExternalSerializer) Flush() []byte {
	i := p.index
	p.index = 0
	return p.buffer[0:i]
}
