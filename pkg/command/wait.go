package command

import (
	"errors"
	"strconv"
	"time"

	"github.com/mhughdo/src/internal/client"
	"github.com/mhughdo/src/pkg/resp"
)

type Wait struct {
	serverInfo ServerInfoProvider
}

func (w *Wait) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 2 {
		return wr.WriteError(errors.New("ERR wrong number of arguments for 'wait' command"))
	}

	numReplicas, err := strconv.Atoi(args[0].String())
	if err != nil || numReplicas < 0 {
		return wr.WriteError(errors.New("ERR invalid number of replicas"))
	}

	timeoutMillis, err := strconv.Atoi(args[1].String())
	if err != nil || timeoutMillis < 0 {
		return wr.WriteError(errors.New("ERR invalid timeout"))
	}

	clientOffset := c.GetLastWriteOffset()
	// If there's no pending writes, return immediately
	if clientOffset == 0 {
		return wr.WriteSimpleValue(resp.Integer, []byte("0"))
	}

	// Start waiting for replicas to acknowledge up to clientOffset
	startTime := time.Now()
	timeout := time.Duration(timeoutMillis) * time.Millisecond
	for {
		ackedReplicas := w.serverInfo.GetReplicaAcknowledgedCount(clientOffset)
		if ackedReplicas >= numReplicas {
			return wr.WriteSimpleValue(resp.Integer, []byte(strconv.Itoa(ackedReplicas)))
		}

		if timeoutMillis == 0 {
			// Block forever, so just sleep briefly
			time.Sleep(10 * time.Millisecond)
			continue
		}

		elapsed := time.Since(startTime)
		if elapsed >= timeout {
			return wr.WriteSimpleValue(resp.Integer, []byte(strconv.Itoa(len(w.serverInfo.GetReplicaInfo()))))
		}

		// Sleep briefly before checking again
		time.Sleep(10 * time.Millisecond)
	}
}

func (w *Wait) IsBlocking(_ []*resp.Resp) bool {
	return true
}
