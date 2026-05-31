package iso8583

type bitmap [16]byte

func newBitmap(data []byte) bitmap {
	var b bitmap
	copy(b[:], data)
	return b
}

// isSet returns true if field n (1-128) is present.
func (b *bitmap) isSet(n int) bool {
	n--
	return (b[n/8]>>(7-(n%8)))&1 == 1
}

func (b *bitmap) set(n int) {
	n--
	b[n/8] |= 1 << (7 - (n % 8))
}

func (b *bitmap) hasSecondary() bool {
	return b.isSet(1)
}

func (b *bitmap) bytes(secondary bool) []byte {
	if secondary {
		return b[:]
	}
	return b[:8]
}
