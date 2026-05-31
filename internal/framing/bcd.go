package framing

import (
	"encoding/binary"
	"fmt"
	"io"
)

type bcdFramer struct {
	headerBytes int
}

func (f *bcdFramer) Read(r io.Reader) ([]byte, error) {
	header := make([]byte, f.headerBytes)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("reading bcd length header: %w", err)
	}

	length, err := f.decode(header)
	if err != nil {
		return nil, err
	}
	if length <= 0 {
		return nil, fmt.Errorf("invalid bcd length header: %d", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("reading payload: %w", err)
	}

	return data, nil
}

func (f *bcdFramer) Write(w io.Writer, data []byte) error {
	header, err := f.encode(len(data))
	if err != nil {
		return err
	}
	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("writing bcd length header: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("writing payload: %w", err)
	}
	return nil
}

func (f *bcdFramer) decode(b []byte) (int, error) {
	switch f.headerBytes {
	case 2:
		return int(binary.BigEndian.Uint16(b)), nil
	case 4:
		return int(binary.BigEndian.Uint32(b)), nil
	}
	return 0, fmt.Errorf("bcd: invalid headerBytes: %d (use 2 or 4)", f.headerBytes)
}

func (f *bcdFramer) encode(length int) ([]byte, error) {
	switch f.headerBytes {
	case 2:
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(length))
		return b, nil
	case 4:
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(length))
		return b, nil
	}
	return nil, fmt.Errorf("bcd: invalid headerBytes: %d (use 2 or 4)", f.headerBytes)
}
