package catmsg

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type ExternalSerializer struct {
	logger     *slog.Logger
	nonce      uint32
	version    Version
	blankKey   []byte
	byteUint64 []byte
	msg        Message
}

func NewExternalSerializer(logger *slog.Logger) *ExternalSerializer {
	s := new(ExternalSerializer)
	s.logger = logger
	s.nonce = 0
	s.version = ProtoclV1
	s.blankKey = make([]byte, 0)
	s.byteUint64 = make([]byte, 8)
	return s
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
func (p *ExternalSerializer) writeHeader(cmdTag CommandTag, bodySize int) ([]byte, error) {
	target := p.msg
	size := headerSize + bodySize
	err := target.Reset(size)
	if err != nil {
		return nil, fmt.Errorf("failed to resize to %d: %s", size, err)
	}
	data := target.Slice()
	if len(data) != size {
		return nil, ErrInsufficientBytes
	}
	i := 0
	data[i] = cmdTag
	i += 1
	data[i] = p.version
	i += 1
	binary.LittleEndian.PutUint32(data[i:i+4], p.nonce)
	p.nonce++
	i += 4

	return data[i:], nil
}

func (p *ExternalSerializer) Ping(t time.Time) (Message, error) {
	slice, err := p.writeHeader(CmdPing, 8)
	if err != nil {
		return p.msg, err
	}
	binary.LittleEndian.PutUint64(slice, uint64(t.Unix()))
	return p.msg, nil
}

func (p *ExternalSerializer) Shutdown() (Message, error) {
	_, err := p.writeHeader(CmdShutdown, 0)
	if err != nil {
		return p.msg, err
	}
	return p.msg, nil
}

type customMessage interface {
	Key() []byte
	Value() []byte
}

func (p *ExternalSerializer) Custom(custom customMessage) (Message, error) {
	key := custom.Key()
	value := custom.Value()
	if key == nil {
		return p.msg, errors.New("blank key")
	}
	if value == nil {
		return p.msg, errors.New("blank value")
	}
	data, err := p.writeHeader(CmdCustom, 1+len(key)+2+len(value))
	if err != nil {
		return p.msg, err
	}

	//	if len(key) == 0 {
	//		return errors.New("blank key")
	//	}
	if MaxKeySize < len(key) {
		return p.msg, fmt.Errorf("key too big: %d vs %d", MaxKeySize, len(key))
	}
	if MaxValueSize < len(value) {
		return p.msg, fmt.Errorf("value too big: %d vs %d", MaxValueSize, len(value))
	}
	i := 0
	data[i] = uint8(len(key))
	i += 1
	copy(data[i:i+len(key)], key[:])
	i += len(key)
	binary.LittleEndian.PutUint16(data[i:i+2], uint16(len(value)))
	i += 2
	copy(data[i:i+len(value)], value[:])
	i += len(value)
	if i != len(data) {
		panic(fmt.Errorf("bad size matching: %d vs %d", i, len(data)))
	}
	return p.msg, nil
}
