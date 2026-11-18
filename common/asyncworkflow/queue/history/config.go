package history

type queueConfig struct{}

func (c *queueConfig) ID() string {
	return "history"
}
