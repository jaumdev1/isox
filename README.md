# isox

A high-performance ISO 8583 message router for issuer-side deployments. Maintains a persistent TCP connection to a network host (e.g. Mastercard MIP), parses incoming authorization requests, and routes them to an HTTP authorizer — with nginx-style configuration, canary deployments, and zero-downtime config reload.

## How it works

```
MIP (Mastercard)           isox              Authorizer
     │                          │                          │
     │  persistent TCP          │                          │
     │ ◄────────────────────────┤                          │
     │                          │                          │
     │  0100 (auth request) ───►│                          │
     │                          │  HTTP POST (JSON)  ─────►│
     │                          │◄───────────────── 200 OK │
     │◄──────────── 0110 ───────│                          │
     │                          │                          │
     │  0800 (heartbeat)   ─── ►│                          │
     │◄──────────── 0810 ───────│  (answered directly,     │
                                    no upstream call)
```

The router connects to the MIP as a TCP client and keeps the connection open indefinitely. Messages flow in both directions over the same connection. The router uses a worker pool and Go channels internally — no goroutine-per-message allocation.

## Features

- **Persistent TCP** — single long-lived connection to the network host with automatic reconnection
- **nginx-style config** — routing rules based on MTI, DE fields, with `==`, `!=`, `starts_with`, `contains`, `regex` operators
- **ISO 8583 → HTTP bridge** — configurable field mapping from ISO 8583 to JSON and back
- **Canary deployments** — weighted traffic splitting across multiple authorizer versions
- **Zero-downtime reload** — send `SIGHUP` to apply a new config without dropping the TCP connection
- **Heartbeat** — automatic `0800`/`0810` handling to keep the MIP connection alive
- **BCD and ASCII framing** — configurable 2 or 4 byte length headers

## Architecture

```
MIP TCP
  │
  ├── connection/client.go    persistent TCP client, auto-reconnect
  │         │
  │    framing/               reads/writes length-prefixed frames (BCD or ASCII)
  │         │
  │    iso8583/               parses bytes → Message{MTI, Fields map[int]string}
  │         │
  │    pipeline/reader        pushes parsed messages into inbound channel
  │
  ├── pipeline/worker (N)     reads inbound channel, evaluates route, calls upstream
  │         │
  │    router/engine          first-match-wins rule evaluation
  │         │
  │    upstream/pool          weighted random upstream selection
  │         │
  │    upstream/http          HTTP POST with ISO 8583 → JSON mapping
  │
  └── pipeline/writer         reads outbound channel, writes frames to TCP
```

`engine` and `pool` are stored as `atomic.Pointer` — a `SIGHUP` swaps them in a single CPU instruction. Workers already in flight finish with the old config; new requests use the new one.

## Configuration

```nginx
global {
    workers         8;
    log_level       info;
    log_file        /var/log/isox/router.log;
    metrics_port    9090;
}

downstream mip_mastercard {
    addr                  192.168.1.100:8583;
    length_header         4;
    length_encoding       bcd;          # bcd or ascii
    reconnect_interval_ms 5000;

    heartbeat {
        interval_ms   30000;
        timeout_ms    5000;
        mti           "0800";
        de[70]        "301";
    }
}

upstream authorizer_stable {
    url        http://authorizer-v1:8080/authorize;
    timeout_ms 20000;

    request_mapping {
        mti     -> body.mti;
        de[2]   -> body.pan;
        de[3]   -> body.processing_code;
        de[4]   -> body.amount;
        de[11]  -> body.stan;
        de[41]  -> body.terminal_id;
        de[42]  -> body.merchant_id;
    }

    response_mapping {
        body.response_code -> de[39];
        body.auth_code     -> de[38];
    }
}

upstream authorizer_canary {
    url        http://authorizer-v2:8080/authorize;
    timeout_ms 20000;
    # ... same mappings
}

route {
    match mti == "0800" {
        action  echo;
        de[39]  "00";
    }

    default {
        upstream authorizer_stable weight 90;
        upstream authorizer_canary weight 10;
    }
}
```

### Route operators

| Operator | Example |
|---|---|
| `==` | `mti == "0100"` |
| `!=` | `de[39] != "00"` |
| `starts_with` | `de[2] starts_with "5411"` |
| `ends_with` | `de[41] ends_with "01"` |
| `contains` | `de[48] contains "TAG01"` |
| `regex` | `de[2] regex "^(54\|55)"` |
| `and` | `mti == "0100" and de[3] starts_with "00"` |

## Getting started

**Requirements:** Go 1.22+

```bash
git clone https://github.com/your-org/isox
cd isox
go build ./...
```

### Running locally with test tools

Open three terminals:

```bash
# terminal 1 — mock HTTP authorizer
go run ./cmd/mockauth -port 8080

# terminal 2 — mock MIP (TCP server the router connects to)
go run ./cmd/sendmsg -bind 0.0.0.0:8583 -mti 0100

# terminal 3 — the router
go run ./cmd/router -config config/isox.conf
```

`sendmsg` listens for the router's TCP connection, sends one message, and prints the response.

### Sending different message types

```bash
# standard authorization
go run ./cmd/sendmsg -mti 0100 -pan 5412345678901234

# heartbeat (answered directly by the router)
go run ./cmd/sendmsg -mti 0800

# insufficient funds (PAN starting with 9999)
go run ./cmd/sendmsg -mti 0100 -pan 9999000000000001

# invalid card (PAN starting with 0000)
go run ./cmd/sendmsg -mti 0100 -pan 0000000000000001
```

### Mock authorizer modes

```bash
# always approve (default)
go run ./cmd/mockauth -port 8080

# always decline — insufficient funds
go run ./cmd/mockauth -port 8080 -response 51

# simulate slow authorizer (3s delay)
go run ./cmd/mockauth -port 8080 -delay 3000

# never respond — triggers DE[39]=68 timeout in router
go run ./cmd/mockauth -port 8080 -timeout
```

## Canary deployments

Update weights in the config file and send `SIGHUP` — no restart, no dropped connections:

```bash
# check router PID
go run ./cmd/router -config config/isox.conf
# router started (pid 48291)

# edit weights
vim config/isox.conf

# reload without downtime
kill -HUP 48291
# config reloaded — routes: 2, upstreams: 2
```

Typical rollout sequence:

```nginx
# day 1 — 10% canary
default {
    upstream authorizer_stable weight 90;
    upstream authorizer_canary weight 10;
}

# day 2 — 50/50
default {
    upstream authorizer_stable weight 50;
    upstream authorizer_canary weight 50;
}

# day 3 — full cutover
default {
    upstream authorizer_canary weight 100;
}
```

If the config file has a syntax error, the reload is rejected and the current config stays active.

## Running tests

```bash
# unit tests
go test ./internal/iso8583/...
go test ./internal/router/...

# integration tests (spins up a fake MIP and fake authorizer in memory)
go test ./internal/pipeline/... -v

# all tests
go test ./...
```

The integration suite includes `TestCanaryDistribution`, which sends 200 messages with an 80/20 weight split and asserts the distribution falls within expected bounds.

## Project structure

```
cmd/
  router/       main entry point
  sendmsg/      mock MIP — TCP server for local testing
  mockauth/     mock HTTP authorizer with configurable response codes

internal/
  iso8583/      message struct, parser, serializer, bitmap, field types
  framing/      length-header framing (BCD and ASCII, 2 or 4 bytes)
  config/       config file structs and nginx-style parser
  router/       rule engine — first-match-wins condition evaluation
  upstream/     HTTP client, ISO 8583 ↔ JSON mapper, weighted pool
  connection/   persistent TCP client, reconnect loop, heartbeat
  pipeline/     reader → workers → writer pipeline with atomic hot reload

config/
  isox.conf    example configuration
```

## Response codes

The router generates `DE[39]=68` (response timeout) automatically when the HTTP authorizer does not respond within `timeout_ms`. Echo responses for `0800` always return `DE[39]=00`.

## License

MIT
