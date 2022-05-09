package status

import (
	"bufio"
	"fmt"
	"io"
)

func NewLineRedirector(cb func(line string)) io.WriteCloser {
	r, w := io.Pipe()

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
	}()

	return w
}

type replaceRReader struct {
	reader *bufio.Reader
	lastR  bool
}

func (r *replaceRReader) Read(p []byte) (int, error) {
	written := 0
	for true {
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

func withColor(c string, s string) string {
	switch c {
	case "red":
		c = "\x1b[31m"
	case "green":
		c = "\x1b[32m"
	case "yellow":
		c = "\x1b[33m"
	}
	return fmt.Sprintf("%s%s\x1b[0m", c, s)
}
