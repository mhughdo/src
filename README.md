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
- XLEN
- XREAD

### Replication

Partial support for master-replica replication.

- SAVE
- PSYNC
- REPLCONF
- WAIT
