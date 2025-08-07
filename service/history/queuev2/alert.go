package queuev2

type (
	// Alert is created by a Monitor when some statistics of the Queue is abnormal
	Alert struct {
		AlertType                            AlertType
		AlertAttributesQueuePendingTaskCount *AlertAttributesQueuePendingTaskCount
	}

	AlertType int

	AlertAttributesQueuePendingTaskCount struct {
		CurrentPendingTaskCount  int
		CriticalPendingTaskCount int
	}
)

const (
	AlertTypeUnspecified AlertType = iota
	AlertTypeQueuePendingTaskCount
)
