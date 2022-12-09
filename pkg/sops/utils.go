package sops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"go.mozilla.org/sops/v3"
	"go.mozilla.org/sops/v3/cmd/sops/formats"
	"os"
)

func IsMaybeSopsFile(s []byte) bool {
	return bytes.Index(s, []byte("sops")) != -1
}

func MaybeDecrypt(decrypter SopsDecrypter, encrypted []byte, inputFormat, outputFormat formats.Format) ([]byte, bool, error) {
	if decrypter == nil {
		return encrypted, false, nil
	}

	if !IsMaybeSopsFile(encrypted) {
		return encrypted, false, nil
	}

	d, err := decrypter.SopsDecryptWithFormat(encrypted, inputFormat, outputFormat)
	if err != nil {
		if errors.Is(err, sops.MetadataNotFound) {
			return encrypted, false, nil
		}
		return nil, false, err
	}
	return d, true, nil
}

func MaybeDecryptFile(decrypter SopsDecrypter, path string) error {
	return MaybeDecryptFileTo(decrypter, path, path)
}

func MaybeDecryptFileTo(decrypter SopsDecrypter, path string, to string) error {
	format := formats.FormatForPath(path)

	file, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	decrypted, encrypted, err := MaybeDecrypt(decrypter, file, format, format)
	if err != nil {
		return fmt.Errorf("failed to decrypt file %s: %w", path, err)
	}
	if !encrypted && path == to {
		return nil
	}

	err = os.WriteFile(to, decrypted, 0o600)
	if err != nil {
		return fmt.Errorf("failed to save decrypted file %s: %w", path, err)
	}
	return nil
}

func MaybeDecryptFileToTmp(ctx context.Context, decrypter SopsDecrypter, path string) (string, error) {
	tmp, err := os.CreateTemp(utils.GetTmpBaseDir(ctx), "sops-decrypt-")
	if err != nil {
		return "", err
	}
	_ = tmp.Close()
	return tmp.Name(), MaybeDecryptFileTo(decrypter, path, tmp.Name())
}
