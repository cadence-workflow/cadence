package metrics

// Code generated ./internal/tools/metricsgen; DO NOT EDIT

func NewServiceTags(
	Hostname string,
	RuntimeEnv string,
) ServiceTags {
	res := ServiceTags{
		Hostname:   Hostname,
		RuntimeEnv: RuntimeEnv,
	}
	return res
}

func (self ServiceTags) NumTags() int {
	num := 2 // num of self fields
	num += 0 // num of reserved fields
	return num
}

func (self ServiceTags) Tags(into map[string]string) {
	into["host"] = self.Hostname
	into["env"] = self.RuntimeEnv
}
