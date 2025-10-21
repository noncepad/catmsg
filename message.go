package catmsg

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
