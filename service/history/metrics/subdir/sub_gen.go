package subdir

// Code generated ./internal/tools/metricsgen; DO NOT EDIT

import (
	"github.com/uber/cadence/service/history/metrics"
)

func NewDoesThisWorkTags(
	ServiceTags metrics.ServiceTags,
	ShardTasklistTags metrics.ShardTasklistTags,
	Something string,
) DoesThisWorkTags {
	res := DoesThisWorkTags{
		ServiceTags:       ServiceTags,
		ShardTasklistTags: ShardTasklistTags,
		Something:         Something,
	}
	return res
}

func (self DoesThisWorkTags) NumTags() int {
	num := 1 // num of self fields
	num += 1 // num of reserved fields
	num += self.ServiceTags.NumTags()
	num += self.ShardTasklistTags.NumTags()
	return num
}

func (self DoesThisWorkTags) Tags(into map[string]string) {
	self.ServiceTags.Tags(into)
	self.ShardTasklistTags.Tags(into)
	into["something"] = self.Something
}
