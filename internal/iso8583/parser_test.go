package iso8583

import (
	"testing"
)

func TestParseSerializeRoundtrip(t *testing.T) {
	original := NewMessage()
	original.MTI = "0100"
	original.Fields[2] = "5412345678901234"
	original.Fields[3] = "000000"
	original.Fields[4] = "000000010000"
	original.Fields[11] = "000042"
	original.Fields[41] = "TERM0001"
	original.Fields[42] = "MERCHANT000001 "

	data, err := Serialize(original)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if parsed.MTI != original.MTI {
		t.Errorf("MTI: esperado %q, obtido %q", original.MTI, parsed.MTI)
	}

	for de, expected := range original.Fields {
		got := parsed.Fields[de]
		if got != expected {
			t.Errorf("DE[%d]: esperado %q, obtido %q", de, expected, got)
		}
	}
}

func TestParseMTI(t *testing.T) {
	msg := NewMessage()
	msg.MTI = "0800"
	msg.Fields[70] = "301"

	data, err := Serialize(msg)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if parsed.MTI != "0800" {
		t.Errorf("MTI: esperado 0800, obtido %q", parsed.MTI)
	}
	if parsed.Fields[70] != "301" {
		t.Errorf("DE[70]: esperado 301, obtido %q", parsed.Fields[70])
	}
}

func TestParseLLVAR(t *testing.T) {
	msg := NewMessage()
	msg.MTI = "0100"
	msg.Fields[2] = "4111111111111111" // PAN — LLVAR

	data, err := Serialize(msg)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if parsed.Fields[2] != "4111111111111111" {
		t.Errorf("DE[2] LLVAR: esperado %q, obtido %q", "4111111111111111", parsed.Fields[2])
	}
}

func TestParseSecondaryBitmap(t *testing.T) {
	// DE[70] está no bitmap secundário (campos 65-128)
	msg := NewMessage()
	msg.MTI = "0800"
	msg.Fields[70] = "301"

	data, err := Serialize(msg)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if _, ok := parsed.Fields[70]; !ok {
		t.Error("DE[70] deveria estar presente mas não está")
	}
}

func TestParseInvalidData(t *testing.T) {
	_, err := Parse([]byte("123")) // menos de 4 bytes
	if err == nil {
		t.Error("esperava erro para dados insuficientes")
	}
}
