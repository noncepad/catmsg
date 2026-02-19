package catmsg

import (
	"bytes"
	"encoding/binary"
	"errors"
)

type Database struct{}

// KVStore holds a fixed number of key-value pairs in one []byte
type KVStore struct {
	nonce uint32
	data  []byte
	slots int
}

const (
	MaxKeySize   = 64
	MaxValueSize = 1024
	EntrySize    = MaxKeySize + 4 + MaxValueSize // 1092 bytes
)

// NewKVStore allocates a new store with N fixed-size slots
func NewKVStore(slots int) *KVStore {
	return &KVStore{
		nonce: 0,
		data:  make([]byte, slots*EntrySize),
		slots: slots,
	}
}

// this number increments on each message sent out to the bot
func (kv *KVStore) Nonce() uint32 {
	return kv.nonce
}

func (kv *KVStore) Iterate(callback func(key, value []byte) error) error {
	var err error
	for slot := 0; slot < kv.slots; slot++ {
		offset := slot * EntrySize
		key := bytes.TrimRight(kv.data[offset:offset+MaxKeySize], "\x00")
		if len(key) == 0 {
			continue // empty slot
		}
		offset += MaxKeySize
		valLen := binary.LittleEndian.Uint32(kv.data[offset : offset+4])
		offset += 4
		value := kv.data[offset : offset+int(valLen)]
		err = callback(key, value)
		if err != nil {
			return err // stop early if callback returns false
		}
	}
	return nil
}

// Put inserts or updates a key/value pair
func (kv *KVStore) Put(key, value []byte, msg targetSlice) error {
	kv.nonce++
	if len(key) > MaxKeySize {
		return errors.New("key too long")
	}
	if len(value) > MaxValueSize {
		return errors.New("value too long")
	}
	slot := kv.findSlot(key)
	offset := slot * EntrySize

	// Write key
	copy(kv.data[offset:offset+MaxKeySize], pad(key, MaxKeySize))
	offset += MaxKeySize

	// Write value length
	binary.LittleEndian.PutUint32(kv.data[offset:offset+4], uint32(len(value)))
	offset += 4

	// Write value
	copy(kv.data[offset:offset+MaxValueSize], pad(value, MaxValueSize))
	var err error
	if msg != nil {
		err = WriteKeyPair(kv.nonce, key, value, msg)
	}
	return err
}

// Get retrieves a value by key
func (kv *KVStore) Get(key []byte) ([]byte, bool) {
	slot := kv.findSlot(key)
	offset := slot * EntrySize
	storedKey := bytes.TrimRight(kv.data[offset:offset+MaxKeySize], "\x00")
	if !bytes.Equal(storedKey, key) {
		return nil, false
	}
	offset += MaxKeySize
	valLen := binary.LittleEndian.Uint32(kv.data[offset : offset+4])
	offset += 4
	value := kv.data[offset : offset+int(valLen)]
	return append([]byte(nil), value...), true
}

// findSlot finds a slot index for a key using simple linear probe
func (kv *KVStore) findSlot(key []byte) int {
	hash := simpleHash(key) % uint32(kv.slots)
	for i := 0; i < kv.slots; i++ {
		slot := (int(hash) + i) % kv.slots
		offset := slot * EntrySize
		storedKey := bytes.TrimRight(kv.data[offset:offset+MaxKeySize], "\x00")
		if len(storedKey) == 0 || bytes.Equal(storedKey, key) {
			return slot
		}
	}
	return 0 // fallback (shouldn't happen if not full)
}

// simpleHash is a small FNV-1a variant
func simpleHash(data []byte) uint32 {
	var h uint32 = 2166136261
	for _, b := range data {
		h ^= uint32(b)
		h *= 16777619
	}
	return h
}

func pad(b []byte, n int) []byte {
	p := make([]byte, n)
	copy(p, b)
	return p
}
