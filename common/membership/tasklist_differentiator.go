package membership

import "regexp"

const uuidRegex = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`

var uuidRegexp = regexp.MustCompile(uuidRegex)

func TaskListExcludedFromShardDistributor(taskListName string) bool {
	// This regex checks if the task list name has a UUID, if it does we
	// consider it a short lived tasklist that will not be managed by the shard distributor.
	return uuidRegexp.MatchString(taskListName)
}
