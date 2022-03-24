package yaml

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/goccy/go-yaml"
	yaml3 "gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func newYamlDecoder(r io.Reader) *yaml.Decoder {
	return yaml.NewDecoder(r, yaml.Strict(), yaml.Validator(Validator))
}

func newYamlEncoder(w io.Writer) *yaml.Encoder {
	return yaml.NewEncoder(w)
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
	var err error
	if _, ok := o.(*map[string]interface{}); ok {
		// much faster
		d := yaml3.NewDecoder(r)
		err = d.Decode(o)
	} else {
		// we need proper working strict mode
		d := newYamlDecoder(r)
		err = d.Decode(o)
	}
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
	// yaml.v3 is much faster then go-yaml
	d := yaml3.NewDecoder(r)

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
