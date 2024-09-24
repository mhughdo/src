# SRC (Simple Redis Clone)

A simple Redis clone written in Go.

## Running the server

- Run as a master: `go run app/server.go`
- Run as a replica:
  ` go run app/server.go --port 6380 --replicaof "localhost 6379"`

## Features

### Supported commands

### String

- SET
- GET
- INCR

### Transactions

- MULTI
- EXEC
- DISCARD

### Generic

- KEYS
- TYPE
- CONFIG

### Connection

- PING
- HELLO
- INFO
- CLIENT

### Streams

- XADD
- XRANGE
- XREAD

### Replication

Partitally support for single leader replication. The following commands are
supported:

- SAVE
- PSYNC
- REPLCONF
- WAIT
