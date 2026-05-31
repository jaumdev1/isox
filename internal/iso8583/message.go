package iso8583

type Message struct {
	MTI    string
	Fields map[int]string
}

func NewMessage() *Message {
	return &Message{
		Fields: make(map[int]string),
	}
}

func (m *Message) Field(de int) (string, bool) {
	v, ok := m.Fields[de]
	return v, ok
}

func (m *Message) MustField(de int) string {
	return m.Fields[de]
}

func (m *Message) SetField(de int, value string) {
	m.Fields[de] = value
}
