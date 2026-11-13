package history

import (
	"fmt"

	"github.com/uber/cadence/common/asyncworkflow/queue/provider"
)

func init() {
	must := func(err error) {
		if err != nil {
			panic(fmt.Errorf("failed to register default provider: %w", err))
		}
	}
	must(provider.RegisterQueueProvider("history", newQueue))
	must(provider.RegisterDecoder("history", newDecoder))
}
