package catmsg

import (
	"encoding/binary"
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
	index      int
	buffer     []byte
}

func NewExternalSerializer(logger *slog.Logger) *ExternalSerializer {
	s := new(ExternalSerializer)
	s.logger = logger
	s.nonce = 1
	s.version = ProtoclV1
	s.blankKey = make([]byte, 0)
	s.byteUint64 = make([]byte, 8)
	s.index = 0
	s.buffer = make([]byte, BUFMAX)
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

// Flush dumps a byte slice of written data. the index resets to 0.
func (p *ExternalSerializer) Flush() []byte {
	i := p.index
	p.index = 0
	return p.buffer[0:i]
}

func (p *ExternalSerializer) CustomRaw(data []byte) error {
	if MaxValueSize-100 < len(data) {
		return fmt.Errorf("too much data: %d vs %d", MaxValueSize-100, len(data))
	}
	p.writeHeader(CmdCustom, 2+len(data))
	{

		slice := p.buffer[p.index:(p.index + 2)]
		p.index += 2
		binary.LittleEndian.PutUint16(slice[:], uint16(len(data)))
	}
	{

		slice := p.buffer[p.index:(p.index + len(data))]
		copy(slice, data)
		p.index += len(data)
	}
	return nil
}
