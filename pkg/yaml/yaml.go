package yaml

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	yaml2 "gopkg.in/yaml.v2"
	yaml3 "gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Decoder interface {
	Decode(v interface{}) error
}

type decoderWrapper struct {
	d *yaml3.Decoder
}

func (w *decoderWrapper) Decode(v interface{}) error {
	err := w.d.Decode(v)
	if err != nil {
		return err
	}

	err = ValidateStructs(v)
	if err != nil {
		return err
	}

	return nil
}

func newDecoder(r io.Reader, out any) Decoder {
	d := yaml3.NewDecoder(r)
	d.KnownFields(true)
	return &decoderWrapper{d: d}
}

func newUnicodeReader(r io.Reader) io.Reader {
	utf16bom := unicode.BOMOverride(unicode.UTF8.NewDecoder())
	return transform.NewReader(r, utf16bom)
}

func ReadYamlFile(p string, o interface{}) error {
	r, err := os.Open(p)
	if err != nil {
		return fmt.Errorf("opening %v failed: %w", p, err)
	}
	defer r.Close()

	err = ReadYamlStream(r, o)
	if err != nil {
		return fmt.Errorf("unmarshalling %v failed: %w", p, err)
	}
	return nil
}

func ReadYamlString(s string, o interface{}) error {
	return ReadYamlStream(strings.NewReader(s), o)
}

func ReadYamlBytes(b []byte, o interface{}) error {
	return ReadYamlStream(bytes.NewReader(b), o)
}

func ReadYamlStream(r io.Reader, o interface{}) error {
	r = newUnicodeReader(r)

	d := newDecoder(r, o)

	err := d.Decode(o)
	if err != nil && errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func ReadYamlAllFile(p string) ([]interface{}, error) {
	r, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("opening %v failed: %w", p, err)
	}
	defer r.Close()

	return ReadYamlAllStream(r)
}

func ReadYamlAllString(s string) ([]interface{}, error) {
	return ReadYamlAllStream(strings.NewReader(s))
}

func ReadYamlAllBytes(b []byte) ([]interface{}, error) {
	return ReadYamlAllStream(bytes.NewReader(b))
}

func ReadYamlAllStream(r io.Reader) ([]interface{}, error) {
	r = newUnicodeReader(r)

	d := newDecoder(r, nil)

	var l []interface{}
	for true {
		var o interface{}
		err := d.Decode(&o)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if o != nil {
			l = append(l, o)
		}
	}

	return l, nil
}

func WriteYamlString(o interface{}) (string, error) {
	return WriteYamlAllString([]interface{}{o})
}

func WriteYamlBytes(o interface{}) ([]byte, error) {
	return WriteYamlAllBytes([]interface{}{o})
}

func WriteYamlFile(p string, o interface{}) error {
	return WriteYamlAllFile(p, []interface{}{o})
}

func WriteYamlAllFile(p string, l []interface{}) error {
	w, err := os.Create(p)
	if err != nil {
		return err
	}
	defer w.Close()

	return WriteYamlAllStream(w, l)
}

func WriteYamlAllBytes(l []interface{}) ([]byte, error) {
	w := bytes.NewBuffer(nil)

	err := WriteYamlAllStream(w, l)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func WriteYamlAllString(l []interface{}) (string, error) {
	b, err := WriteYamlAllBytes(l)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func WriteYamlAllStream(w io.Writer, l []interface{}) error {
	enc := yaml3.NewEncoder(w)
	defer enc.Close()

	enc.SetIndent(2)

	for _, o := range l {
		err := enc.Encode(o)
		if err != nil {
			return err
		}
	}
	return nil
}

func WriteJsonString(o interface{}) (string, error) {
	x, err := WriteYamlBytes(o)
	if err != nil {
		return "", err
	}

	x, err = ConvertYamlToJson(x)
	if err != nil {
		return "", err
	}
	return string(x), nil
}

func ConvertYamlToJson(b []byte) ([]byte, error) {
	var x interface{}
	err := ReadYamlBytes(b, &x)
	if err != nil {
		return nil, err
	}
	b, err = json.Marshal(x)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// RemoveDuplicateFields is a helper/hack to remove duplicate fields from yaml maps/structs. The yaml spec explicitly
// forbids duplicate keys, but yaml.v2 ignored those by default, leading to some tools (e.g. Helm) ignoring these. This
// forces us to also ignore/remove them in some cases. We do this by loading the yaml via yaml.v2 and then writing them
// back to a string which can then be parsed by yaml.v3
// TODO Remove this helper method when https://github.com/go-yaml/yaml/issues/751 is implemented
func RemoveDuplicateFields(r io.Reader) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	d := yaml2.NewDecoder(r)
	e := yaml2.NewEncoder(buf)

	for {
		var o interface{}
		err := d.Decode(&o)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if o != nil {
			err = e.Encode(o)
			if err != nil {
				return nil, err
			}
		}
	}
	return buf.Bytes(), nil
}

func FixNameExt(dir string, name string) string {
	p := filepath.Join(dir, name)
	p = FixPathExt(p)
	return filepath.Base(p)
}

func FixPathExt(p string) string {
	if utils.Exists(p) {
		return p
	}
	var p2 string
	if strings.HasSuffix(p, ".yml") {
		p2 = p[:len(p)-4] + ".yaml"
	} else if strings.HasSuffix(p, ".yaml") {
		p2 = p[:len(p)-5] + ".yml"
	} else {
		return p
	}

	if utils.Exists(p2) {
		return p2
	}
	return p
}

func Exists(p string) bool {
	p = FixPathExt(p)
	return utils.Exists(p)
}
