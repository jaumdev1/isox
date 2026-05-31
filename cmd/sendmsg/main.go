package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"sort"
	"time"

	"github.com/isox/internal/framing"
	"github.com/isox/internal/iso8583"
)

func main() {
	bind     := flag.String("bind", "0.0.0.0:8583", "address to listen on (simulates MIP)")
	mti      := flag.String("mti", "0100", "message MTI")
	pan      := flag.String("pan", "5412345678901234", "DE[2] PAN")
	amount   := flag.String("amount", "000000010000", "DE[4] amount")
	stan     := flag.String("stan", "000001", "DE[11] STAN")
	terminal := flag.String("terminal", "TERM0001", "DE[41] terminal ID")
	encoding := flag.String("encoding", "bcd", "length header encoding (bcd or ascii)")
	header   := flag.Int("header", 4, "length header size in bytes (2 or 4)")
	flag.Parse()

	framer, err := framing.New(*encoding, *header)
	if err != nil {
		log.Fatalf("framer: %v", err)
	}

	ln, err := net.Listen("tcp", *bind)
	if err != nil {
		log.Fatalf("listen on %s: %v", *bind, err)
	}
	defer ln.Close()

	fmt.Printf("mock MIP listening on %s\n", *bind)
	fmt.Printf("waiting for router to connect...\n\n")

	conn, err := ln.Accept()
	if err != nil {
		log.Fatalf("accept: %v", err)
	}
	defer conn.Close()

	fmt.Printf("router connected: %s\n\n", conn.RemoteAddr())

	msg := buildMessage(*mti, *pan, *amount, *stan, *terminal)
	data, err := iso8583.Serialize(msg)
	if err != nil {
		log.Fatalf("serialize: %v", err)
	}

	fmt.Printf("sending MTI=%s...\n", *mti)
	if err := framer.Write(conn, data); err != nil {
		log.Fatalf("write: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	respData, err := framer.Read(conn)
	if err != nil {
		log.Fatalf("waiting for response: %v", err)
	}

	resp, err := iso8583.Parse(respData)
	if err != nil {
		log.Fatalf("parse response: %v", err)
	}

	fmt.Printf("\nresponse received:\n")
	fmt.Printf("  MTI     = %s\n", resp.MTI)

	des := make([]int, 0, len(resp.Fields))
	for de := range resp.Fields {
		des = append(des, de)
	}
	sort.Ints(des)
	for _, de := range des {
		fmt.Printf("  DE[%3d] = %s\n", de, resp.Fields[de])
	}
}

func buildMessage(mti, pan, amount, stan, terminal string) *iso8583.Message {
	msg := iso8583.NewMessage()
	msg.MTI = mti

	switch mti {
	case "0100", "0200":
		msg.Fields[2]  = pan
		msg.Fields[3]  = "000000"
		msg.Fields[4]  = amount
		msg.Fields[11] = stan
		msg.Fields[12] = time.Now().Format("150405")
		msg.Fields[13] = time.Now().Format("0102")
		msg.Fields[41] = terminal
		msg.Fields[42] = "MERCHANT000001 "
		msg.Fields[49] = "986"
	case "0800":
		msg.Fields[11] = stan
		msg.Fields[70] = "301"
	}

	return msg
}
