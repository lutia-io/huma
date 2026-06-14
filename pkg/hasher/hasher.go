package hasher

type Hasher interface {
	Hash(text string) (string, error)
}
