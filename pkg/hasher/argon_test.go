package hasher

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"
)

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, errors.New("read failed") }

func TestNewArgon2IDHasher_defaults(t *testing.T) {
	h := NewArgon2IDHasher()
	ah, ok := h.(*argon2IDHasher)
	if !ok {
		t.Fatalf("expected *argon2IDHasher, got %T", h)
	}
	if ah.time != 3 {
		t.Fatalf("time: expected 3, got %d", ah.time)
	}
	if ah.memoryKiB != 64*1024 {
		t.Fatalf("memoryKiB: expected %d, got %d", 64*1024, ah.memoryKiB)
	}
	if ah.threads != 4 {
		t.Fatalf("threads: expected 4, got %d", ah.threads)
	}
	if ah.keyLen != 32 || ah.saltLen != 16 {
		t.Fatalf("unexpected key/salt lens: keyLen=%d saltLen=%d", ah.keyLen, ah.saltLen)
	}
}

func TestArgon2IDHasher_Hash_readSaltError(t *testing.T) {
	old := rand.Reader
	rand.Reader = errReader{}
	defer func() { rand.Reader = old }()

	h := NewArgon2IDHasher()
	_, err := h.Hash("pw")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestArgon2IDHasher_Hash_success_formatAndB64(t *testing.T) {
	h := NewArgon2IDHasher()
	got, err := h.Hash("pw")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.HasPrefix(got, "argon2id$v=19$m=") {
		t.Fatalf("unexpected prefix: %q", got)
	}

	parts := strings.Split(got, "$")
	if len(parts) != 5 {
		t.Fatalf("unexpected format: %q", got)
	}

	saltB64 := parts[3]
	hashB64 := parts[4]
	salt, err := base64.RawStdEncoding.DecodeString(saltB64)
	if err != nil {
		t.Fatalf("salt not b64: %v", err)
	}
	key, err := base64.RawStdEncoding.DecodeString(hashB64)
	if err != nil {
		t.Fatalf("key not b64: %v", err)
	}
	if len(salt) != 16 {
		t.Fatalf("expected salt len 16, got %d", len(salt))
	}
	if len(key) != 32 {
		t.Fatalf("expected key len 32, got %d", len(key))
	}
}

var _ io.Reader = errReader{}
