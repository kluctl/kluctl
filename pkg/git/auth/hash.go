package auth

import (
	"crypto/sha256"
	"encoding/binary"
	"golang.org/x/crypto/ssh"
)

type hashProvider interface {
	Hash() ([]byte, error)
}

func buildHash(s ssh.Signer) ([]byte, error) {
	if hp, ok := s.(hashProvider); ok {
		return hp.Hash()
	}
	h := sha256.New()
	_ = binary.Write(h, binary.LittleEndian, "pk")
	_ = binary.Write(h, binary.LittleEndian, s.PublicKey().Marshal())
	return h.Sum(nil), nil
}

func buildHashForList(signers []ssh.Signer) ([]byte, error) {
	h := sha256.New()
	_ = binary.Write(h, binary.LittleEndian, "signers")
	for _, s := range signers {
		h2, err := buildHash(s)
		if err != nil {
			return nil, err
		}
		_ = binary.Write(h, binary.LittleEndian, h2)
	}
	return h.Sum(nil), nil
}
