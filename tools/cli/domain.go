// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cli

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/tools/common/commoncli"
)

// by default we don't require any domain data. But this can be overridden by calling SetRequiredDomainDataKeys()
var requiredDomainDataKeys = []string{}

// SetRequiredDomainDataKeys will set requiredDomainDataKeys
func SetRequiredDomainDataKeys(keys []string) {
	requiredDomainDataKeys = keys
}

func checkRequiredDomainDataKVs(domainData map[string]string) error {
	// check requiredDomainDataKeys
	for _, k := range requiredDomainDataKeys {
		_, ok := domainData[k]
		if !ok {
			return fmt.Errorf("domain data error, missing required key %v . All required keys: %v", k, requiredDomainDataKeys)
		}
	}
	return nil
}

func checkNoAdditionalArgsPassed(c *cli.Context) error {
	if c.NArg() > 0 {
		return fmt.Errorf("Domain commands cannot have arguments: <%v>\nClusters are now specified as --clusters c1,c2 see help for more info", strings.Join(c.Args().Slice(), " "))
	}
	return nil
}

func newDomainCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "register",
			Aliases: []string{"re"},
			Usage:   "Register workflow domain",
			Flags:   registerDomainFlags,
			Action: func(c *cli.Context) error {
				err := checkNoAdditionalArgsPassed(c)
				if err != nil {
					return err
				}
				return withDomainClient(c, false, func(dc *domainCLIImpl) error {
					return dc.RegisterDomain(c)
				})
			},
		},
		{
			Name:    "update",
			Aliases: []string{"up", "u"},
			Usage:   "Update existing workflow domain",
			Flags:   updateDomainFlags,
			Action: func(c *cli.Context) error {
				err := checkNoAdditionalArgsPassed(c)
				if err != nil {
					return err
				}
				return withDomainClient(c, false, func(dc *domainCLIImpl) error {
					return dc.UpdateDomain(c)
				})
			},
		},
		{
			Name:    "delete",
			Aliases: []string{"del"},
			Usage:   "Delete existing workflow domain",
			Flags:   deleteDomainFlags,
			Action: func(c *cli.Context) error {
				err := checkNoAdditionalArgsPassed(c)
				if err != nil {
					return err
				}
				return withDomainClient(c, false, func(dc *domainCLIImpl) error {
					return dc.DeleteDomain(c)
				})
			},
		},
		{
			Name:    "deprecate",
			Aliases: []string{"dep"},
			Usage:   "Deprecate existing workflow domain",
			Flags:   deprecateDomainFlags,
			Action: func(c *cli.Context) error {
				err := checkNoAdditionalArgsPassed(c)
				if err != nil {
					return err
				}
				return withDomainClient(c, false, func(dc *domainCLIImpl) error {
					return dc.DeprecateDomain(c)
				})
			},
		},
		{
			Name:    "describe",
			Aliases: []string{"desc"},
			Usage:   "Describe existing workflow domain",
			Flags:   describeDomainFlags,
			Action: func(c *cli.Context) error {
				err := checkNoAdditionalArgsPassed(c)
				if err != nil {
					return err
				}
				return withDomainClient(c, false, func(dc *domainCLIImpl) error {
					return dc.DescribeDomain(c)
				})
			},
		},
		{
			Name:    "migration",
			Aliases: []string{"mi"},
			Usage:   "Migrate existing domain to new domain. This command only validates the settings. It does not perform actual data migration",
			Flags:   migrateDomainFlags,
			Action: func(c *cli.Context) error {
				err := checkNoAdditionalArgsPassed(c)
				if err != nil {
					return err
				}
				// exit on error already handled in the command
				// TODO best practice is to return error if validation fails
				err = NewDomainMigrationCommand(c).Validation(c)
				if err != nil {
					return commoncli.Problem("Failed validation: ", err)
				}
				return nil
			},
		},
		{
			Name:    "failover",
			Aliases: []string{"fo"},
			Usage:   "Failover workflow domain to target active cluster",
			Flags:   failoverDomainFlags,
			Action: func(c *cli.Context) error {
				err := checkNoAdditionalArgsPassed(c)
				if err != nil {
					return err
				}
				return withDomainClient(c, false, func(dc *domainCLIImpl) error {
					return dc.FailoverDomain(c)
				})
			},
		},
		{
			Name:    "list-failover-history",
			Aliases: []string{"lfh"},
			Usage:   "List failover history for a domain",
			Flags:   listFailoverHistoryFlags,
			Action: func(c *cli.Context) error {
				err := checkNoAdditionalArgsPassed(c)
				if err != nil {
					return err
				}
				return withDomainClient(c, false, func(dc *domainCLIImpl) error {
					return dc.ListFailoverHistory(c)
				})
			},
		},
	}
}
