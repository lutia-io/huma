package hasher

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"

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

func (h *argon2IDHasher) Compare(text, encodedHash string) (bool, error) {
	memoryKiB, time, threads, salt, key, err := parseArgon2IDHash(encodedHash)
	if err != nil {
		return false, err
	}
	derived := argon2.IDKey([]byte(text), salt, time, memoryKiB, threads, uint32(len(key)))
	if subtle.ConstantTimeCompare(derived, key) == 1 {
		return true, nil
	}
	return false, nil
}

func parseArgon2IDHash(encodedHash string) (memoryKiB, time uint32, threads uint8, salt, key []byte, err error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 5 || parts[0] != "argon2id" {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid argon2id hash format")
	}
	if parts[1] != "v=19" {
		return 0, 0, 0, nil, nil, fmt.Errorf("unsupported argon2id version")
	}
	var m, t, p int
	for _, param := range strings.Split(parts[2], ",") {
		kv := strings.SplitN(param, "=", 2)
		if len(kv) != 2 {
			return 0, 0, 0, nil, nil, fmt.Errorf("invalid argon2id params")
		}
		n, convErr := strconv.Atoi(kv[1])
		if convErr != nil {
			return 0, 0, 0, nil, nil, fmt.Errorf("invalid argon2id param value: %w", convErr)
		}
		switch kv[0] {
		case "m":
			m = n
		case "t":
			t = n
		case "p":
			p = n
		default:
			return 0, 0, 0, nil, nil, fmt.Errorf("unknown argon2id param %q", kv[0])
		}
	}
	if m <= 0 || t <= 0 || p <= 0 {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid argon2id params")
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid argon2id salt: %w", err)
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid argon2id key: %w", err)
	}
	return uint32(m), uint32(t), uint8(p), salt, key, nil
}
