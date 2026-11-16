//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination poller_disconnector_mock.go -package matching github.com/uber/cadence/client/matching PollerDisconnector

package matching

import "context"

// PollerDisconnector cancels the polls in flight and makes the client return empty responses to new polls.
// It is meant for the graceful shutdown of a service which polls matching on behalf of workers.
type PollerDisconnector interface {
	DisconnectPollers(ctx context.Context)
}
