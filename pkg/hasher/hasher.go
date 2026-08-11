package hasher

type Hasher interface {
	Hash(text string) (string, error)
	// Compare reports whether text matches the encoded hash.
	Compare(text, encodedHash string) (bool, error)
}
