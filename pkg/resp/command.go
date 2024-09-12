package resp

import "bytes"

func CreateCommand(args ...string) []byte {
	var buf bytes.Buffer
	w := NewWriter(&buf, RESP3)
	w.WriteStringSlice(args)

	return buf.Bytes()
}

func CreatePingCommand() []byte {
	return CreateCommand("PING")
}

func CreateReplconfCommand(args ...string) []byte {
	return CreateCommand(append([]string{"REPLCONF"}, args...)...)
}

func CreatePsyncCommand(replicationID string, offset string) []byte {
	return CreateCommand("PSYNC", replicationID, offset)
}
