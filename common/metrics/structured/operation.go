package structured

import "github.com/uber/cadence/common/metrics"

func GetOperationString(op int) string {
	for _, serviceOps := range metrics.ScopeDefs {
		if serviceOp, ok := serviceOps[op]; ok {
			return serviceOp.GetOperationString() // unit test ensures this is unique
		}
	}
	return "unknown_operation_int" // should never happen
}
