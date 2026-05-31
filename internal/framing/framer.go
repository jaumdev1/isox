package framing

import (
	"fmt"
	"io"
)

type Framer interface {
	Read(r io.Reader) ([]byte, error)
	Write(w io.Writer, data []byte) error
}

func New(encoding string, headerBytes int) (Framer, error) {
	switch encoding {
	case "ascii":
		return &asciiFramer{headerBytes: headerBytes}, nil
	case "bcd":
		return &bcdFramer{headerBytes: headerBytes}, nil
	default:
		return nil, fmt.Errorf("unknown framing encoding: %q (use ascii or bcd)", encoding)
	}
}
