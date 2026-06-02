package failovermanager

import (
	"strings"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

// isDomainEligibleForFailover is the shared gating check used by both failover and rebalance.
// It excludes domains that:
//   - are nil or have no DomainInfo
//   - are not global (failover is a multi-cluster operation)
//   - have an explicit Deprecated/Deleted status (no work to do)
//   - are not opted in via the managed-failover domain-data key
//
// A nil Status field is treated as the zero value (Registered) to match the legacy behaviour
// of shouldFailover/shouldAllowRebalance — frontends do not always populate Status on every
// DescribeDomainResponse, and dropping those domains would silently break failover.
func isDomainEligibleForFailover(domain *types.DescribeDomainResponse) bool {
	if domain == nil {
		return false
	}

	if domain.DomainInfo == nil {
		return false
	}

	switch domain.DomainInfo.GetStatus() {
	case types.DomainStatusDeprecated, types.DomainStatusDeleted:
		return false
	}

	if !domain.GetIsGlobalDomain() {
		return false
	}

	if !isDomainFailoverManagedByCadence(domain) {
		return false
	}

	return true
}

func isDomainFailoverManagedByCadence(domain *types.DescribeDomainResponse) bool {
	domainData := domain.DomainInfo.GetData()
	return strings.ToLower(strings.TrimSpace(domainData[constants.DomainDataKeyForManagedFailover])) == "true"
}
