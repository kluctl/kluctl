package term

import (
	"io"
	"runtime"
)

func ReadLineNoEcho(fd int, cb func(ret []byte)) ([]byte, error) {
	return readLineNoEcho(fd, cb)
}

func readLine(reader io.Reader, cb func(ret []byte)) ([]byte, error) {
	var buf [1]byte
	var ret []byte

	for {
		n, err := reader.Read(buf[:])
		if n > 0 {
			switch buf[0] {
			case '':
				// ignore ESC sequences
				continue
			case '', '\b':
				if len(ret) > 0 {
					ret = ret[:len(ret)-1]
				}
				if cb != nil {
					cb(ret)
				}
			case '\n':
				if runtime.GOOS != "windows" {
					return ret, nil
				}
				// otherwise ignore \n
			case '\r':
				if runtime.GOOS == "windows" {
					return ret, nil
				}
				// otherwise ignore \r
			default:
				ret = append(ret, buf[0])
				if cb != nil {
					cb(ret)
				}
			}
			continue
		}
		if err != nil {
			if err == io.EOF && len(ret) > 0 {
				return ret, nil
			}
			return ret, err
		}
	}
}
