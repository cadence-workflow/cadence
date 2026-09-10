package config

import (
	"fmt"
	"strings"
	"time"
)

// BootstrapMode is an enum type for ringpop bootstrap mode
type BootstrapMode int

const (
	// BootstrapModeNone represents a bootstrap mode set to nothing or invalid
	BootstrapModeNone BootstrapMode = iota
	// BootstrapModeFile represents a file-based bootstrap mode
	BootstrapModeFile
	// BootstrapModeHosts represents a list of hosts passed in the configuration
	BootstrapModeHosts
	// BootstrapModeCustom represents a custom bootstrap mode
	BootstrapModeCustom
	// BootstrapModeDNS represents a list of hosts passed in the configuration
	// to be resolved, and the resulting addresses are used for bootstrap
	BootstrapModeDNS
	// BootstrapModeDNSSRV represents a list of DNS hosts passed in the configuration
	// to resolve secondary addresses that DNS SRV record would return resulting in
	// a host list that will contain multiple dynamic addresses and their unique ports
	BootstrapModeDNSSRV
)

const (
	defaultMaxJoinDuration = 10 * time.Second
)

// Config contains the ringpop config items
type Config struct {
	// Name to be used in ringpop advertisement
	Name string `yaml:"name"`
	// BroadcastAddress is communicated with peers to connect to this container/host.
	// This is useful when running cadence in K8s and the containers need to listen on 0.0.0.0 but advertise their pod IP to peers.
	// If not set, the listen address will be used as broadcast address.
	BroadcastAddress string `yaml:"broadcastAddress"`
	// BootstrapMode is a enum that defines the ringpop bootstrap method, currently supports: hosts, files, custom, dns, and dns-srv
	BootstrapMode BootstrapMode `yaml:"bootstrapMode"`
	// BootstrapHosts is a list of seed hosts to be used for ringpop bootstrap
	BootstrapHosts []string `yaml:"bootstrapHosts"`
	// BootstrapFile is the file path to be used for ringpop bootstrap
	BootstrapFile string `yaml:"bootstrapFile"`
	// MaxJoinDuration is the max wait time to join the ring
	MaxJoinDuration time.Duration `yaml:"maxJoinDuration"`
}

// IsEmpty returns true if all config fields are zero values
func (rpConfig *Config) IsEmpty() bool {
	return rpConfig.Name == "" &&
		rpConfig.BroadcastAddress == "" &&
		rpConfig.BootstrapMode == 0 &&
		len(rpConfig.BootstrapHosts) == 0 &&
		rpConfig.BootstrapFile == "" &&
		rpConfig.MaxJoinDuration == 0
}

func (rpConfig *Config) Validate() error {
	if len(rpConfig.Name) == 0 {
		return fmt.Errorf("ringpop config missing `name` param")
	}

	if rpConfig.MaxJoinDuration == 0 {
		rpConfig.MaxJoinDuration = defaultMaxJoinDuration
	}

	return validateBootstrapMode(rpConfig)
}

// UnmarshalYAML is called by the yaml package to convert
// the config YAML into a BootstrapMode.
func (m *BootstrapMode) UnmarshalYAML(
	unmarshal func(interface{}) error,
) error {

	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	var err error
	*m, err = parseBootstrapMode(s)
	return err
}

// parseBootstrapMode reads a string value and returns a bootstrap mode.
func parseBootstrapMode(
	mode string,
) (BootstrapMode, error) {

	switch strings.ToLower(mode) {
	case "hosts":
		return BootstrapModeHosts, nil
	case "file":
		return BootstrapModeFile, nil
	case "custom":
		return BootstrapModeCustom, nil
	case "dns":
		return BootstrapModeDNS, nil
	case "dns-srv":
		return BootstrapModeDNSSRV, nil
	}
	return BootstrapModeNone, fmt.Errorf("invalid ringpop bootstrap mode %q", mode)
}

func validateBootstrapMode(
	rpConfig *Config,
) error {

	switch rpConfig.BootstrapMode {
	case BootstrapModeFile:
		if len(rpConfig.BootstrapFile) == 0 {
			return fmt.Errorf("ringpop config missing bootstrap file param")
		}
	case BootstrapModeHosts, BootstrapModeDNS, BootstrapModeDNSSRV:
		if len(rpConfig.BootstrapHosts) == 0 {
			return fmt.Errorf("ringpop config missing boostrap hosts param")
		}
	case BootstrapModeCustom:
		return fmt.Errorf("ringpop bootstrapMode custom is not supported")
	default:
		return fmt.Errorf("ringpop config with unknown boostrap mode %q", rpConfig.BootstrapMode)
	}
	return nil
}
