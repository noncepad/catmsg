package catmsg

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const BUFMAX int = 4 * 1024

type Message struct {
	size      int
	buffer    [BUFMAX]byte
	readIndex int
}

func (kvm *Message) Reset(size int) {
	kvm.size = size
	kvm.readIndex = 0
}

func (kvm *Message) Size() int {
	return kvm.size
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

type CommandTag = uint8

const (
	CMD_PAIR CommandTag = 1
	CMD_DUMP CommandTag = 2
)

type targetSlice interface {
	Reset(size int)
	Slice() []byte
}

func WriteKeyPair(version uint32, key, value []byte, target targetSlice) error {
	// The binary format is [CMD, 1B][version,4B uint32][key_size,1B uint8][key,?B][value_size,2B uint16][value,?B]
	size := 1 + 4 + 1 + len(key) + 2 + len(value)
	target.Reset(size)
	data := target.Slice()
	if len(data) != size {
		return ErrInsufficientBytes
	}
	if len(key) == 0 {
		return errors.New("blank key")
	}
	if MaxKeySize < len(key) {
		return fmt.Errorf("key too big: %d vs %d", MaxKeySize, len(key))
	}
	if MaxValueSize < len(value) {
		return fmt.Errorf("value too big: %d vs %d", MaxValueSize, len(value))
	}
	i := 0
	data[i] = CMD_PAIR
	i += 1
	binary.LittleEndian.PutUint32(data[i:i+4], version)
	i += 4
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
	return nil
}

// MessageFromKeyValue converts a key value pair to a single message.
// The binary format is [CMD, 1B][version,4B uint32][key_size,1B uint8][key,?B][value_size,2B uint16][value,?B]
func MessageFromKeyValue(version uint32, key, value []byte, msg *Message) error {
	if len(key) == 0 {
		return errors.New("blank key")
	}
	if MaxKeySize < len(key) {
		return fmt.Errorf("key too big: %d vs %d", MaxKeySize, len(key))
	}
	if MaxValueSize < len(value) {
		return fmt.Errorf("value too big: %d vs %d", MaxValueSize, len(value))
	}
	i := 0
	msg.buffer[i] = CMD_PAIR
	i += 1
	binary.LittleEndian.PutUint32(msg.buffer[i:i+4], version)
	i += 4
	msg.buffer[i] = uint8(len(key))
	i += 1
	copy(msg.buffer[i:i+len(key)], key[:])
	i += len(key)
	binary.LittleEndian.PutUint16(msg.buffer[i:i+2], uint16(len(value)))
	i += 2
	copy(msg.buffer[i:i+len(value)], value[:])
	i += len(value)
	msg.size = i
	return nil
}
