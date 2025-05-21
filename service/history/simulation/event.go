package simulation

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

var enabled = false

func init() {
	enabled = os.Getenv("HISTORY_LOG_EVENTS") == "true"
}

const (
	EventNameCreateHistoryTask  = "Create History Task"
	EventNameExecuteHistoryTask = "Execute History Task"
)

type E struct {
	ShardID    int
	DomainID   string
	WorkflowID string
	RunID      string

	EventTime time.Time

	// EventName describes the event. It is used to query events in simulations so don't change existing event names.
	EventName string
	Host      string
	Payload   map[string]any
}

func LogEvents(events ...E) {
	if !enabled {
		return
	}
	for _, e := range events {
		e.EventTime = time.Now()
		data, err := json.Marshal(e)
		if err != nil {
			fmt.Printf("failed to marshal event: %v", err)
		}

		fmt.Printf("History New Event: %s\n", data)
	}
}
