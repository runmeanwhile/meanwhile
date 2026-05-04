package anthropic

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

type sseDecoder struct {
	reader *bufio.Reader
}

func newSSEDecoder(r io.Reader) *sseDecoder {
	return &sseDecoder{reader: bufio.NewReader(r)}
}

func (d *sseDecoder) Next() ([]byte, error) {
	var data bytes.Buffer
	for {
		line, err := d.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && data.Len() > 0 {
				break
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if data.Len() == 0 {
				continue
			}
			break
		}
		if after, ok := strings.CutPrefix(line, "data:"); ok {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(after))
		}
	}
	return bytes.TrimSpace(data.Bytes()), nil
}
