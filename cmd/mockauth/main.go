package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

var (
	port            = flag.String("port", "8080", "listening port")
	defaultResponse = flag.String("response", "00", "default DE[39] (00=approved, 51=insufficient funds, 14=invalid card)")
	delayMs         = flag.Int("delay", 0, "response delay in ms")
	alwaysTimeout   = flag.Bool("timeout", false, "never respond (tests router timeout handling)")
)

type Request struct {
	Body struct {
		MTI            string `json:"mti"`
		PAN            string `json:"pan"`
		ProcessingCode string `json:"processing_code"`
		Amount         string `json:"amount"`
		STAN           string `json:"stan"`
		LocalTime      string `json:"local_time"`
		LocalDate      string `json:"local_date"`
		TerminalID     string `json:"terminal_id"`
		MerchantID     string `json:"merchant_id"`
		CurrencyCode   string `json:"currency_code"`
	} `json:"body"`
}

type Response struct {
	Body struct {
		ResponseCode string `json:"response_code"`
		AuthCode     string `json:"auth_code"`
	} `json:"body"`
}

func main() {
	flag.Parse()

	http.HandleFunc("/authorize", handleAuthorize)
	http.HandleFunc("/health", handleHealth)

	addr := fmt.Sprintf(":%s", *port)
	log.Printf("mockauth started on %s", addr)
	log.Printf("  default response : DE[39]=%s", *defaultResponse)
	log.Printf("  delay            : %dms", *delayMs)
	log.Printf("  timeout mode     : %v", *alwaysTimeout)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("invalid request body: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	log.Printf("received: MTI=%s STAN=%s PAN=%s AMOUNT=%s TERMINAL=%s",
		req.Body.MTI,
		req.Body.STAN,
		maskPAN(req.Body.PAN),
		req.Body.Amount,
		req.Body.TerminalID,
	)

	if *alwaysTimeout {
		log.Printf("  → timeout mode: holding response forever")
		select {}
	}

	if *delayMs > 0 {
		log.Printf("  → waiting %dms...", *delayMs)
		time.Sleep(time.Duration(*delayMs) * time.Millisecond)
	}

	responseCode, authCode := decide(req)
	log.Printf("  → DE[39]=%s DE[38]=%s", responseCode, authCode)

	resp := Response{}
	resp.Body.ResponseCode = responseCode
	resp.Body.AuthCode = authCode

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func decide(req Request) (responseCode, authCode string) {
	if len(req.Body.PAN) >= 4 {
		switch req.Body.PAN[:4] {
		case "0000":
			return "14", ""
		case "9999":
			return "51", ""
		case "8888":
			return "62", ""
		}
	}

	code := *defaultResponse
	auth := ""
	if code == "00" {
		auth = fmt.Sprintf("A%05s", req.Body.STAN)
	}
	return code, auth
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func maskPAN(pan string) string {
	if len(pan) < 8 {
		return "****"
	}
	masked := pan[:6]
	for i := 6; i < len(pan)-4; i++ {
		masked += "*"
	}
	return masked + pan[len(pan)-4:]
}
