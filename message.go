package catmsg

import (
	"encoding/binary"
)

const (
	BUFMAX int = 4 * 1024
)

type Message struct {
	size      int
	buffer    [BUFMAX]byte
	readIndex int
	nonce     uint32
}

type FixedPair struct {
	lenK  int
	key   [MaxKeySize]byte
	lenV  int
	value [MaxValueSize]byte
}

func (fp *FixedPair) Key() []byte {
	return fp.key[:fp.lenK]
}

func (fp *FixedPair) Value() []byte {
	return fp.value[:fp.lenV]
}

func (kvm *Message) Reset(size int) error {
	kvm.size = size
	kvm.readIndex = 0
	if len(kvm.buffer) < kvm.size {
		return ErrInsufficientBytes
	}
	return nil
}

func (kvm *Message) Size() int {
	return kvm.size
}

func (kvm *Message) Nonce(nonce uint32) {
	kvm.nonce = nonce
}

func (kvm *Message) SliceWithNonce(nonce uint32) []byte {
	data := kvm.buffer[0:kvm.size]
	i := 1
	binary.LittleEndian.PutUint32(data[i:i+4], kvm.nonce)
	return kvm.buffer[0:kvm.size]
}

func (kvm *Message) Slice() []byte {
	return kvm.buffer[0:kvm.size]
}

func (kvm *Message) Read(data []byte) (n int, err error) {
	n = min(len(data), kvm.size-kvm.readIndex)
	copy(data[0:n], kvm.buffer[kvm.readIndex:kvm.readIndex+n])
	kvm.readIndex += n
	return n, nil
}

type Version = uint8

const (
	ProtoclV1 Version = 1
)

type CommandTag = uint8

const (
	// internal -> external
	CmdDump   CommandTag = 1
	CmdPong   CommandTag = 2
	CmdCustom CommandTag = 3
	CmdPubkey CommandTag = 4
	// external -> internal
	CmdPing     CommandTag = 5
	CmdShutdown CommandTag = 6
)

type targetSlice interface {
	Reset(size int) error
	Slice() []byte
}
