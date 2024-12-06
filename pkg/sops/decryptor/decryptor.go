/*
Copyright 2020 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package decryptor

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	intkeyservice "github.com/kluctl/kluctl/v2/pkg/sops/keyservice"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/getsops/sops/v3"
	"github.com/getsops/sops/v3/aes"
	"github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/cmd/sops/common"
	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/config"
	"github.com/getsops/sops/v3/keys"
	"github.com/getsops/sops/v3/keyservice"
	"github.com/getsops/sops/v3/pgp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/resource"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

const (
	// MaxEncryptedFileSize is the max allowed file size in bytes of an encrypted
	// file.
	MaxEncryptedFileSize int64 = 5 << 20
	// unsupportedFormat is used to signal no sopsFormatToMarkerBytes format was
	// detected by detectFormatFromMarkerBytes.
	unsupportedFormat = formats.Format(-1)
)

var (
	// sopsFormatToString is the counterpart to
	// https://github.com/mozilla/sops/blob/v3.7.2/cmd/sops/formats/formats.go#L16
	sopsFormatToString = map[formats.Format]string{
		formats.Binary: "binary",
		formats.Dotenv: "dotenv",
		formats.Ini:    "INI",
		formats.Json:   "JSON",
		formats.Yaml:   "YAML",
	}
	// sopsFormatToMarkerBytes contains a list of formats and their byte
	// order markers, used to detect if a Secret data field is SOPS' encrypted.
	sopsFormatToMarkerBytes = map[formats.Format][]byte{
		// formats.Binary is a JSON envelop at encrypted rest
		formats.Binary: []byte("\"mac\": \"ENC["),
		formats.Dotenv: []byte("sops_mac=ENC["),
		formats.Ini:    []byte("[sops]"),
		formats.Json:   []byte("\"mac\": \"ENC["),
		formats.Yaml:   []byte("mac: ENC["),
	}
)

// Decryptor performs decryption operations for a
// kluctlv1.KluctlDeployment.
// The only supported decryption provider at present is
// DecryptionProviderSOPS.
type Decryptor struct {
	// root is the root for file system operations. Any (relative) path or
	// symlink is not allowed to traverse outside this path.
	root string
	// maxFileSize is the max size in bytes a file is allowed to have to be
	// decrypted. Defaults to MaxEncryptedFileSize.
	maxFileSize int64
	// checkSopsMac instructs the decryptor to perform the SOPS data integrity
	// check using the MAC. Not enabled by default, as arbitrary data gets
	// injected into most resources, causing the integrity check to fail.
	// Mostly kept around for feature completeness and documentation purposes.
	checkSopsMac bool

	// keyServices are the SOPS keyservice.KeyServiceClient's available to the
	// decryptor.
	keyServices []keyservice.KeyServiceClient
}

// NewDecryptor creates a new Decryptor for the given kluctlDeployment.
// gnuPGHome can be empty, in which case the systems' keyring is used.
func NewDecryptor(root string, maxFileSize int64) *Decryptor {
	return &Decryptor{
		root:        root,
		maxFileSize: maxFileSize,
	}
}

func (d *Decryptor) AddLocalKeyService() {
	d.AddKeyServiceClient(keyservice.NewLocalClient())
}

func (d *Decryptor) AddKeyServiceClient(s keyservice.KeyServiceClient) {
	d.keyServices = append(d.keyServices, s)
}

// IsEncryptedSecret checks if the given object is a Kubernetes Secret encrypted
// with Mozilla SOPS.
func IsEncryptedSecret(object *unstructured.Unstructured) bool {
	if object.GetKind() == "Secret" && object.GetAPIVersion() == "v1" {
		if _, found, _ := unstructured.NestedFieldNoCopy(object.Object, "sops"); found {
			return true
		}
	}
	return false
}

// IsOfflineMethod returns true for offline decrypt methods or false otherwise
func IsOfflineMethod(mk keys.MasterKey) bool {
	switch mk.(type) {
	case *pgp.MasterKey, *age.MasterKey:
		return true
	default:
		return false
	}
}

// SopsDecryptWithFormat attempts to load a SOPS encrypted file using the store
// for the input format, gathers the data key for it from the key service,
// and then decrypts the file data with the retrieved data key.
// It returns the decrypted bytes in the provided output format, or an error.
func (d *Decryptor) SopsDecryptWithFormat(data []byte, inputFormat, outputFormat formats.Format) (_ []byte, err error) {
	defer func() {
		// It was discovered that malicious input and/or output instructions can
		// make SOPS panic. Recover from this panic and return as an error.
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to emit encrypted %s file as decrypted %s: %v",
				sopsFormatToString[inputFormat], sopsFormatToString[outputFormat], r)
		}
	}()

	store := common.StoreForFormat(inputFormat, config.NewStoresConfig())

	tree, err := store.LoadEncryptedFile(data)
	if err != nil {
		return nil, sopsUserErr(fmt.Sprintf("failed to load encrypted %s data", sopsFormatToString[inputFormat]), err)
	}

	for _, group := range tree.Metadata.KeyGroups {
		// Sort MasterKeys in the group so offline ones are tried first
		sort.SliceStable(group, func(i, j int) bool {
			return IsOfflineMethod(group[i]) && !IsOfflineMethod(group[j])
		})
	}

	for _, group := range tree.Metadata.KeyGroups {
		// Sort MasterKeys in the group so offline ones are tried first
		sort.SliceStable(group, func(i, j int) bool {
			return intkeyservice.IsOfflineMethod(group[i]) && !intkeyservice.IsOfflineMethod(group[j])
		})
	}

	metadataKey, err := tree.Metadata.GetDataKeyWithKeyServices(d.keyServices, sops.DefaultDecryptionOrder)
	if err != nil {
		return nil, sopsUserErr("cannot get sops data key", err)
	}

	cipher := aes.NewCipher()
	mac, err := tree.Decrypt(metadataKey, cipher)
	if err != nil {
		return nil, sopsUserErr("error decrypting sops tree", err)
	}

	if d.checkSopsMac {
		// Compute the hash of the cleartext tree and compare it with
		// the one that was stored in the document. If they match,
		// integrity was preserved
		// Ref: github.com/getsops/sops/v3/decrypt/decrypt.go
		originalMac, err := cipher.Decrypt(
			tree.Metadata.MessageAuthenticationCode,
			metadataKey,
			tree.Metadata.LastModified.Format(time.RFC3339),
		)
		if err != nil {
			return nil, sopsUserErr("failed to verify sops data integrity", err)
		}
		if originalMac != mac {
			// If the file has an empty MAC, display "no MAC"
			if originalMac == "" {
				originalMac = "no MAC"
			}
			return nil, fmt.Errorf("failed to verify sops data integrity: expected mac '%s', got '%s'", originalMac, mac)
		}
	}

	outputStore := common.StoreForFormat(outputFormat, config.NewStoresConfig())
	out, err := outputStore.EmitPlainFile(tree.Branches)
	if err != nil {
		return nil, sopsUserErr(fmt.Sprintf("failed to emit encrypted %s file as decrypted %s",
			sopsFormatToString[inputFormat], sopsFormatToString[outputFormat]), err)
	}
	return out, err
}

// DecryptResource attempts to decrypt the provided resource overwriting the resource
// with the decrypted data.
// It has special support for Kubernetes Secrets with encrypted data entries
// while decrypting with DecryptionProviderSOPS, to allow individual data entries
// injected by e.g. a Kustomize secret generator to be decrypted
func (d *Decryptor) DecryptResource(res *resource.Resource) (*resource.Resource, error) {
	if res == nil {
		return nil, nil
	}

	switch {
	case isSOPSEncryptedResource(res):
		// As we are expecting to decrypt right before applying, we do not
		// care about keeping any other data (e.g. comments) around.
		// We can therefore simply work with JSON, which saves us from e.g.
		// JSON -> YAML -> JSON transformations.
		out, err := res.MarshalJSON()
		if err != nil {
			return nil, err
		}

		data, err := d.SopsDecryptWithFormat(out, formats.Json, formats.Json)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt and format '%s/%s' %s data: %w",
				res.GetNamespace(), res.GetName(), res.GetKind(), err)
		}

		err = res.UnmarshalJSON(data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal decrypted '%s/%s' %s to JSON: %w",
				res.GetNamespace(), res.GetName(), res.GetKind(), err)
		}
		return res, nil
	case res.GetKind() == "Secret":
		dataMap := res.GetDataMap()
		for key, value := range dataMap {
			data, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				// If we fail to base64 decode, it is (very) likely to be a
				// user input error. Instead of failing here, let it bubble
				// up during the actual build.
				continue
			}

			if inF := detectFormatFromMarkerBytes(data); inF != unsupportedFormat {
				outF := formatForPath(key)
				out, err := d.SopsDecryptWithFormat(data, inF, outF)
				if err != nil {
					return nil, fmt.Errorf("failed to decrypt and format '%s/%s' Secret field '%s': %w",
						res.GetNamespace(), res.GetName(), key, err)
				}
				dataMap[key] = base64.StdEncoding.EncodeToString(out)
			}
		}
		res.SetDataMap(dataMap)
		return res, nil
	}
	return nil, nil
}

// DecryptEnvSources attempts to decrypt all types.SecretArgs FileSources and
// EnvSources a Kustomization file in the directory at the provided path refers
// to, before walking recursively over all other resources it refers to.
// It ignores resource references which refer to absolute or relative paths
// outside the working directory of the decryptor, but returns any decryption
// error.
func (d *Decryptor) DecryptEnvSources(path string) error {
	decrypted, visited := make(map[string]struct{}, 0), make(map[string]struct{}, 0)
	visit := d.decryptKustomizationEnvSources(decrypted)
	return recurseKustomizationFiles(d.root, path, visit, visited)
}

// decryptKustomizationEnvSources returns a visitKustomization implementation
// which attempts to decrypt any EnvSources entry it finds in the Kustomization
// file with which it is called.
// After decrypting successfully, it adds the absolute path of the file to the
// given map.
func (d *Decryptor) decryptKustomizationEnvSources(visited map[string]struct{}) visitKustomization {
	return func(root, path string, kus *kustypes.Kustomization) error {
		visitRef := func(sourcePath string, format formats.Format) error {
			if !filepath.IsAbs(sourcePath) {
				sourcePath = filepath.Join(path, sourcePath)
			}
			absRef, _, err := securePaths(root, sourcePath)
			if err != nil {
				return err
			}
			if _, ok := visited[absRef]; ok {
				return nil
			}

			if err := d.sopsDecryptFile(absRef, format, format); err != nil {
				return securePathErr(root, err)
			}

			// Explicitly set _after_ the decryption operation, this makes
			// visited work as a list of actually decrypted files
			visited[absRef] = struct{}{}
			return nil
		}

		for _, gen := range kus.SecretGenerator {
			for _, fileSrc := range gen.FileSources {
				parts := strings.SplitN(fileSrc, "=", 2)
				key := parts[0]
				var filePath string
				if len(parts) > 1 {
					filePath = parts[1]
				} else {
					filePath = key
				}
				if err := visitRef(filePath, formatForPath(key)); err != nil {
					return err
				}
			}
			for _, envFile := range gen.EnvSources {
				format := formatForPath(envFile)
				if format == formats.Binary {
					// Default to dotenv
					format = formats.Dotenv
				}
				if err := visitRef(envFile, format); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

// sopsDecryptFile attempts to decrypt the file at the given path using SOPS'
// store for the provided input format, and writes it back to the path using
// the store for the output format.
// Path must be absolute and a regular file, the file is not allowed to exceed
// the maxFileSize.
//
// NB: The method only does the simple checks described above and does not
// verify whether the path provided is inside the working directory. Boundary
// enforcement is expected to have been done by the caller.
func (d *Decryptor) sopsDecryptFile(path string, inputFormat, outputFormat formats.Format) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if !fi.Mode().IsRegular() {
		return fmt.Errorf("cannot decrypt irregular file as it has file mode type bits set")
	}
	if fileSize := fi.Size(); d.maxFileSize > 0 && fileSize > d.maxFileSize {
		return fmt.Errorf("cannot decrypt file with size (%d bytes) exceeding limit (%d)", fileSize, d.maxFileSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if !bytes.Contains(data, sopsFormatToMarkerBytes[inputFormat]) {
		return nil
	}

	out, err := d.SopsDecryptWithFormat(data, inputFormat, outputFormat)
	if err != nil {
		return err
	}
	err = os.WriteFile(path, out, 0o644)
	if err != nil {
		return fmt.Errorf("error writing sops decrypted %s data to %s file: %w",
			sopsFormatToString[inputFormat], sopsFormatToString[outputFormat], err)
	}
	return nil
}

// sopsEncryptWithFormat attempts to load a plain file using the store
// for the input format, gathers the data key for it from the key service,
// and then encrypt the file data with the retrieved data key.
// It returns the encrypted bytes in the provided output format, or an error.
func (d *Decryptor) sopsEncryptWithFormat(metadata sops.Metadata, data []byte, inputFormat, outputFormat formats.Format) ([]byte, error) {
	store := common.StoreForFormat(inputFormat, config.NewStoresConfig())

	branches, err := store.LoadPlainFile(data)
	if err != nil {
		return nil, err
	}

	tree := sops.Tree{
		Branches: branches,
		Metadata: metadata,
	}
	dataKey, errs := tree.GenerateDataKeyWithKeyServices(d.keyServices)
	if len(errs) > 0 {
		return nil, sopsUserErr("could not generate data key", fmt.Errorf("%s", errs))
	}

	cipher := aes.NewCipher()
	unencryptedMac, err := tree.Encrypt(dataKey, cipher)
	if err != nil {
		return nil, sopsUserErr("error encrypting sops tree", err)
	}
	tree.Metadata.LastModified = time.Now().UTC()
	tree.Metadata.MessageAuthenticationCode, err = cipher.Encrypt(unencryptedMac, dataKey, tree.Metadata.LastModified.Format(time.RFC3339))
	if err != nil {
		return nil, sopsUserErr("cannot encrypt sops data tree", err)
	}

	outStore := common.StoreForFormat(outputFormat, config.NewStoresConfig())
	out, err := outStore.EmitEncryptedFile(tree)
	if err != nil {
		return nil, sopsUserErr("failed to emit sops encrypted file", err)
	}
	return out, nil
}

// secureLoadKustomizationFile tries to securely load a Kustomization file from
// the given directory path.
// If multiple Kustomization files are found, or the request is ambiguous, an
// error is returned.
func secureLoadKustomizationFile(root, path string) (*kustypes.Kustomization, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("root '%s' must be absolute", root)
	}
	if filepath.IsAbs(path) {
		return nil, fmt.Errorf("path '%s' must be relative", path)
	}

	var loadPath string
	for _, fName := range konfig.RecognizedKustomizationFileNames() {
		fPath, err := securejoin.SecureJoin(root, filepath.Join(path, fName))
		if err != nil {
			return nil, fmt.Errorf("failed to secure join %s: %w", fName, err)
		}
		fi, err := os.Lstat(fPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("failed to lstat %s: %w", fName, securePathErr(root, err))
		}

		if !fi.Mode().IsRegular() {
			return nil, fmt.Errorf("expected %s to be a regular file", fName)
		}
		if loadPath != "" {
			return nil, fmt.Errorf("found multiple kustomization files")
		}
		loadPath = fPath
	}
	if loadPath == "" {
		return nil, fmt.Errorf("no kustomization file found")
	}

	data, err := os.ReadFile(loadPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read kustomization file: %w", securePathErr(root, err))
	}

	kus := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
	}
	if err := yaml.Unmarshal(data, &kus); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kustomization file from '%s': %w", loadPath, err)
	}
	return &kus, nil
}

// visitKustomization is called by recurseKustomizationFiles after every
// successful Kustomization file load.
type visitKustomization func(root, path string, kus *kustypes.Kustomization) error

// errRecurseIgnore is a wrapping error to signal to recurseKustomizationFiles
// the error can be ignored during recursion. For example, because the
// Kustomization file can not be loaded for a subsequent call.
type errRecurseIgnore struct {
	Err error
}

// Unwrap returns the actual underlying error.
func (e *errRecurseIgnore) Unwrap() error {
	return e.Err
}

// Error returns the error string of the underlying error.
func (e *errRecurseIgnore) Error() string {
	if err := e.Err; err != nil {
		return e.Err.Error()
	}
	return "recurse ignore"
}

// recurseKustomizationFiles attempts to recursively load and visit
// Kustomization files.
// The provided path is allowed to be relative, in which case it is safely
// joined with root. When absolute, it must be inside root.
func recurseKustomizationFiles(root, path string, visit visitKustomization, visited map[string]struct{}) error {
	// Resolve the secure paths
	absPath, relPath, err := securePaths(root, path)
	if err != nil {
		return err
	}

	if _, ok := visited[absPath]; ok {
		// Short-circuit
		return nil
	}
	visited[absPath] = struct{}{}

	// Confirm we are dealing with a directory
	fi, err := os.Lstat(absPath)
	if err != nil {
		err = securePathErr(root, err)
		if errors.Is(err, fs.ErrNotExist) {
			err = &errRecurseIgnore{Err: err}
		}
		return err
	}
	if !fi.IsDir() {
		return &errRecurseIgnore{Err: fmt.Errorf("not a directory")}
	}

	// Attempt to load the Kustomization file from the directory
	kus, err := secureLoadKustomizationFile(root, relPath)
	if err != nil {
		return err
	}

	// Visit the Kustomization
	if err = visit(root, path, kus); err != nil {
		return err
	}

	// Components may contain resources as well, ...
	// ...so we have to process both .resources and .components values
	resources := append(kus.Resources, kus.Components...)

	// Recurse over other resources in Kustomization,
	// repeating the above logic per item
	for _, res := range resources {
		if !filepath.IsAbs(res) {
			res = filepath.Join(path, res)
		}
		if err = recurseKustomizationFiles(root, res, visit, visited); err != nil {
			// When the resource does not exist at the compiled path, it's
			// either an invalid reference, or a URL.
			// If the reference is valid but does not point to a directory,
			// we have run into a dead end as well.
			// In all other cases, the error is of (possible) importance to
			// the user, and we should return it.
			if _, ok := err.(*errRecurseIgnore); !ok {
				return err
			}
		}
	}
	return nil
}

// isSOPSEncryptedResource detects if the given resource is a SOPS' encrypted
// resource by looking for ".sops" and ".sops.mac" fields.
func isSOPSEncryptedResource(res *resource.Resource) bool {
	if res == nil {
		return false
	}
	sopsField := res.Field("sops")
	if sopsField.IsNilOrEmpty() {
		return false
	}
	macField := sopsField.Value.Field("mac")
	return !macField.IsNilOrEmpty()
}

// securePaths returns the absolute and relative paths for the provided path,
// guaranteed to be scoped inside the provided root.
// When the given path is absolute, the root is stripped before secure joining
// it on root.
func securePaths(root, path string) (string, string, error) {
	if filepath.IsAbs(path) {
		path = stripRoot(root, path)
	}
	secureAbsPath, err := securejoin.SecureJoin(root, path)
	if err != nil {
		return "", "", err
	}
	return secureAbsPath, stripRoot(root, secureAbsPath), nil
}

func stripRoot(root, path string) string {
	sepStr := string(filepath.Separator)
	root, path = filepath.Clean(sepStr+root), filepath.Clean(sepStr+path)
	switch {
	case path == root:
		path = sepStr
	case root == sepStr:
		// noop
	case strings.HasPrefix(path, root+sepStr):
		path = strings.TrimPrefix(path, root+sepStr)
	}
	return filepath.Clean(filepath.Join("."+sepStr, path))
}

func sopsUserErr(msg string, err error) error {
	if userErr, ok := err.(sops.UserError); ok {
		err = fmt.Errorf(userErr.UserError())
	}
	return fmt.Errorf("%s: %w", msg, err)
}

func securePathErr(root string, err error) error {
	if pathErr := new(fs.PathError); errors.As(err, &pathErr) {
		err = &fs.PathError{Op: pathErr.Op, Path: stripRoot(root, pathErr.Path), Err: pathErr.Err}
	}
	return err
}

func formatForPath(path string) formats.Format {
	switch {
	case strings.HasSuffix(path, corev1.DockerConfigJsonKey):
		return formats.Json
	default:
		return formats.FormatForPath(path)
	}
}

func detectFormatFromMarkerBytes(b []byte) formats.Format {
	for k, v := range sopsFormatToMarkerBytes {
		if bytes.Contains(b, v) {
			return k
		}
	}
	return unsupportedFormat
}
