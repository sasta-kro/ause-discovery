package identity

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

func NewUUIDv7(now time.Time) ([16]byte, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return value, err
	}
	milliseconds := uint64(now.UTC().UnixMilli())
	value[0] = byte(milliseconds >> 40)
	value[1] = byte(milliseconds >> 32)
	value[2] = byte(milliseconds >> 24)
	value[3] = byte(milliseconds >> 16)
	value[4] = byte(milliseconds >> 8)
	value[5] = byte(milliseconds)
	value[6] = (value[6] & 0x0f) | 0x70
	value[8] = (value[8] & 0x3f) | 0x80
	return value, nil
}

func Milliseconds(value [16]byte) uint64 {
	return binary.BigEndian.Uint64(append([]byte{0, 0}, value[:6]...))
}
