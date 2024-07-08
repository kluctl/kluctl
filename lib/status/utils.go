package status

import (
	"bufio"
	"io"
)

type LineRedirector struct {
	io.WriteCloser
	done chan struct{}
}

func (lr *LineRedirector) Close() error {
	err := lr.WriteCloser.Close()
	if err != nil {
		return err
	}
	return nil
}

func (lr *LineRedirector) Done() <-chan struct{} {
	return lr.done
}

func NewLineRedirector(cb func(line string)) *LineRedirector {
	r, w := io.Pipe()
	done := make(chan struct{})

	go func() {
		br := bufio.NewReader(r)
		scanner := bufio.NewScanner(&replaceRReader{reader: br})
		for scanner.Scan() {
			msg := scanner.Text()
			if msg == "" {
				continue
			}
			cb(msg)
		}
		close(done)
	}()

	return &LineRedirector{w, done}
}

type replaceRReader struct {
	reader *bufio.Reader
	lastR  bool
}

func (r *replaceRReader) Read(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		b, err := r.reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				if written == 0 {
					return 0, err
				}
				return written, nil
			}
			return 0, err
		}

		if b == '\r' {
			p[written] = '\n'
			written++
			r.lastR = true
			break
		} else if b == '\n' {
			if r.lastR {
				continue
			}
			p[written] = '\n'
			written++
			r.lastR = false
			break
		} else {
			p[written] = b
			written++
			r.lastR = false
		}
	}
	return written, nil
}
