package metrics

// Code generated ./internal/tools/metricsgen; DO NOT EDIT

import (
	"fmt"
)

func NewShardTasklistTags(
	ServiceTags ServiceTags,
	Shard int,
	Type TasklistType,
) ShardTasklistTags {
	res := ShardTasklistTags{
		ServiceTags: ServiceTags,
		Shard:       Shard,
		Type:        Type,
	}
	return res
}

func (self ShardTasklistTags) NumTags() int {
	num := 2 // num of self fields
	num += 1 // num of reserved fields
	num += self.ServiceTags.NumTags()
	return num
}

func (self ShardTasklistTags) Tags(into map[string]string) {
	self.ServiceTags.Tags(into)
	into["shard"] = fmt.Sprintf("%d", self.Shard)
	into["tasklist_type"] = self.Type.String()
}
