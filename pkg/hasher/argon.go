package hasher

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

type argon2IDHasher struct {
	time      uint32
	memoryKiB uint32
	threads   uint8
	keyLen    uint32
	saltLen   int
}

func NewArgon2IDHasher() Hasher {
	return &argon2IDHasher{
		time:      3,
		memoryKiB: 64 * 1024,
		threads:   4,
		keyLen:    32,
		saltLen:   16,
	}
}

func (h *argon2IDHasher) Hash(text string) (string, error) {
	salt := make([]byte, h.saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}

	key := argon2.IDKey([]byte(text), salt, h.time, h.memoryKiB, h.threads, h.keyLen)
	return fmt.Sprintf(
		"argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		h.memoryKiB,
		h.time,
		h.threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}
