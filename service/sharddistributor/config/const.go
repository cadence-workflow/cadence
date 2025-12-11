package config

import "github.com/uber/cadence/common/types"

const (
	NamespaceTypeFixed     = "fixed"
	NamespaceTypeEphemeral = "ephemeral"
)

const (
	MigrationModeINVALID                = "invalid"
	MigrationModeLOCALPASSTHROUGH       = "local_pass"
	MigrationModeLOCALPASSTHROUGHSHADOW = "local_pass_shadow"
	MigrationModeDISTRIBUTEDPASSTHROUGH = "distributed_pass"
	MigrationModeONBOARDED              = "onboarded"
)

// MigrationMode maps string migration mode values to types.MigrationMode
var MigrationMode = map[string]types.MigrationMode{
	MigrationModeINVALID:                types.MigrationModeINVALID,
	MigrationModeLOCALPASSTHROUGH:       types.MigrationModeLOCALPASSTHROUGH,
	MigrationModeLOCALPASSTHROUGHSHADOW: types.MigrationModeLOCALPASSTHROUGHSHADOW,
	MigrationModeDISTRIBUTEDPASSTHROUGH: types.MigrationModeDISTRIBUTEDPASSTHROUGH,
	MigrationModeONBOARDED:              types.MigrationModeONBOARDED,
}
