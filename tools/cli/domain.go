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

	"github.com/urfave/cli"
)

// by default we don't require any domain data. But this can be overridden by calling SetRequiredDomainDataKeys()
var requiredDomainDataKeys = []string{}

// SetRequiredDomainDataKeys will set requiredDomainDataKeys
func SetRequiredDomainDataKeys(keys []string) {
	requiredDomainDataKeys = keys
}

func checkRequiredDomainDataKVs(domainData map[string]string) error {
	//check requiredDomainDataKeys
	for _, k := range requiredDomainDataKeys {
		_, ok := domainData[k]
		if !ok {
			return fmt.Errorf("domain data error, missing required key %v . All required keys: %v", k, requiredDomainDataKeys)
		}
	}
	return nil
}

func parseDomainDataKVs(domainDataStr string) (map[string]string, error) {
	kvstrs := strings.Split(domainDataStr, ",")
	kvMap := map[string]string{}
	for _, kvstr := range kvstrs {
		kv := strings.Split(kvstr, ":")
		if len(kv) != 2 {
			return kvMap, fmt.Errorf("domain data format error. It must be k1:v2,k2:v2,...,kn:vn")
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		kvMap[k] = v
	}

	return kvMap, nil
}

func newDomainCommands() []cli.Command {
	return []cli.Command{
		{
			Name:    "register",
			Aliases: []string{"re"},
			Usage:   "Register workflow domain",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  FlagDescriptionWithAlias,
					Usage: "Domain description",
				},
				cli.StringFlag{
					Name:  FlagOwnerEmailWithAlias,
					Usage: "Owner email",
				},
				cli.StringFlag{
					Name:  FlagRetentionDaysWithAlias,
					Usage: "Workflow execution retention in days",
				},
				cli.StringFlag{
					Name:  FlagEmitMetricWithAlias,
					Usage: "Flag to emit metric",
				},
				cli.StringFlag{
					Name:  FlagActiveClusterNameWithAlias,
					Usage: "Active cluster name",
				},
				cli.StringFlag{ // use StringFlag instead of buggy StringSliceFlag
					Name:  FlagClustersWithAlias,
					Usage: "Clusters",
				},
				cli.StringFlag{
					Name:  FlagIsGlobalDomainWithAlias,
					Usage: "Flag to indicate whether domain is a global domain",
				},
				cli.StringFlag{
					Name:  FlagDomainDataWithAlias,
					Usage: "Domain data of key value pairs, in format of k1:v1,k2:v2,k3:v3",
				},
				cli.StringFlag{
					Name:  FlagSecurityTokenWithAlias,
					Usage: "Security token with permission",
				},
				cli.StringFlag{
					Name:  FlagArchivalStatusWithAlias,
					Usage: "Flag to set archival status, valid values are \"disabled\" and \"enabled\"",
				},
				cli.StringFlag{
					Name:  FlagArchivalBucketNameWithAlias,
					Usage: "Optionally specify bucket (cannot be changed after first time archival is enabled)",
				},
			},
			Action: func(c *cli.Context) {
				RegisterDomain(c)
			},
		},
		{
			Name:    "update",
			Aliases: []string{"up", "u"},
			Usage:   "Update existing workflow domain",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  FlagDescriptionWithAlias,
					Usage: "Domain description",
				},
				cli.StringFlag{
					Name:  FlagOwnerEmailWithAlias,
					Usage: "Owner email",
				},
				cli.StringFlag{
					Name:  FlagRetentionDaysWithAlias,
					Usage: "Workflow execution retention in days",
				},
				cli.StringFlag{
					Name:  FlagEmitMetricWithAlias,
					Usage: "Flag to emit metric",
				},
				cli.StringFlag{
					Name:  FlagActiveClusterNameWithAlias,
					Usage: "Active cluster name",
				},
				cli.StringFlag{ // use StringFlag instead of buggy StringSliceFlag
					Name:  FlagClustersWithAlias,
					Usage: "Clusters",
				},
				cli.StringFlag{
					Name:  FlagDomainDataWithAlias,
					Usage: "Domain data of key value pairs, in format of k1:v1,k2:v2,k3:v3 ",
				},
				cli.StringFlag{
					Name:  FlagSecurityTokenWithAlias,
					Usage: "Security token with permission ",
				},
				cli.StringFlag{
					Name:  FlagArchivalStatusWithAlias,
					Usage: "Flag to set archival status, valid values are \"disabled\" and \"enabled\"",
				},
				cli.StringFlag{
					Name:  FlagArchivalBucketNameWithAlias,
					Usage: "Optionally specify bucket (cannot be changed after first time archival is enabled)",
				},
			},
			Action: func(c *cli.Context) {
				UpdateDomain(c)
			},
		},
		{
			Name:    "describe",
			Aliases: []string{"desc"},
			Usage:   "Describe existing workflow domain",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  FlagDomainID,
					Usage: "Domain UUID (required if not specify domainName)",
				},
			},
			Action: func(c *cli.Context) {
				DescribeDomain(c)
			},
		},
	}
}
