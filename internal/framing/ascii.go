package framing

import (
	"fmt"
	"io"
	"strconv"
)

type asciiFramer struct {
	headerBytes int
}

func (f *asciiFramer) Read(r io.Reader) ([]byte, error) {
	header := make([]byte, f.headerBytes)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("reading ascii length header: %w", err)
	}

	length, err := strconv.Atoi(string(header))
	if err != nil {
		return nil, fmt.Errorf("invalid ascii length header %q: %w", header, err)
	}
	if length <= 0 {
		return nil, fmt.Errorf("invalid ascii length header: %d", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("reading payload: %w", err)
	}

	return data, nil
}

func (f *asciiFramer) Write(w io.Writer, data []byte) error {
	header := fmt.Sprintf(fmt.Sprintf("%%0%dd", f.headerBytes), len(data))

	if _, err := w.Write([]byte(header)); err != nil {
		return fmt.Errorf("writing ascii length header: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("writing payload: %w", err)
	}
	return nil
}
