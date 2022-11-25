package sops

import (
	"fmt"
	"go.mozilla.org/sops/v3/cmd/sops/formats"
	"go.mozilla.org/sops/v3/decrypt"
)

type SopsDecrypter interface {
	SopsDecryptWithFormat(data []byte, inputFormat, outputFormat formats.Format) ([]byte, error)
}

type LocalSopsDecrypter struct {
}

func (_ LocalSopsDecrypter) SopsDecryptWithFormat(data []byte, inputFormat, outputFormat formats.Format) ([]byte, error) {
	if inputFormat != outputFormat {
		return nil, fmt.Errorf("inputFormat and outputFormat must be equal")
	}
	return decrypt.DataWithFormat(data, inputFormat)
}
