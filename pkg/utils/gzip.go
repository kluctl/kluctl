package utils

import (
	"bytes"
	"compress/gzip"
	"io"
)

func CompressGzip(input []byte, level int) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, len(input)))
	w, err := gzip.NewWriterLevel(buf, level)
	if err != nil {
		return nil, err
	}
	_, err = w.Write(input)
	if err != nil {
		return nil, err
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func UncompressGzip(input []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, r)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
