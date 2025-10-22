package catmsg

import "errors"

var (
	ErrInsufficientBytes = errors.New("insufficient bytes")
	ErrKeyTooBig         = errors.New("key is too big")
	ErrValueTooBig       = errors.New("value is too big")
)
