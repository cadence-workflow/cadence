// The MIT License (MIT)
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/reconciliation/store"
	"github.com/uber/cadence/service/worker/scanner/executions"
	"github.com/uber/cadence/tools/common/commoncli"
)

// AdminDBClean is the command to clean up unhealthy executions.
// Input is a JSON stream provided via STDIN or a file.
func AdminDBClean(c *cli.Context) error {
	flagscantype, err := getRequiredOption(c, FlagScanType)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	scanType, err := executions.ScanTypeString(flagscantype)

	if err != nil {
		return commoncli.Problem("unknown scan type", err)
	}
	collectionSlice := c.StringSlice(FlagInvariantCollection)
	blob := scanType.ToBlobstoreEntity()

	var collections []invariant.Collection
	for _, v := range collectionSlice {
		collection, err := invariant.CollectionString(v)
		if err != nil {
			return commoncli.Problem("unknown invariant collection", err)
		}
		collections = append(collections, collection)
	}

	logger := zap.NewNop()
	if c.Bool(FlagVerbose) {
		logger, err = zap.NewDevelopment()
		if err != nil {
			// probably impossible with default config
			return commoncli.Problem("could not construct logger", err)
		}
	}

	invariants := scanType.ToInvariants(collections, logger)
	if len(invariants) < 1 {
		return commoncli.Problem(
			fmt.Sprintf("no invariants for scantype %q and collections %q",
				scanType.String(),
				collectionSlice),
			nil,
		)
	}

	input, err := getInputFile(c.String(FlagInputFile))
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	dec := json.NewDecoder(input)
	var data []*store.ScanOutputEntity

	for {
		soe := &store.ScanOutputEntity{
			Execution: blob.Clone(),
		}

		if err := dec.Decode(&soe); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
		} else {
			data = append(data, soe)
		}
	}

	for _, e := range data {
		result, err := fixExecution(c, invariants, e)
		if err != nil {
			return commoncli.Problem("Error in fix execution: ", err)
		}
		out := store.FixOutputEntity{
			Execution: e.Execution,
			Input:     *e,
			Result:    result,
		}
		data, err := json.Marshal(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			continue
		}

		output := getDeps(c).Output()
		output.Write([]byte(string(data) + "\n"))
	}
	return nil
}

func fixExecution(
	c *cli.Context,
	invariants []executions.InvariantFactory,
	execution *store.ScanOutputEntity,
) (invariant.ManagerFixResult, error) {
	execManager, err := getDeps(c).initializeExecutionManager(c, execution.Execution.(entity.Entity).GetShardID())
	if err != nil {
		return invariant.ManagerFixResult{}, err
	}
	defer execManager.Close()

	historyV2Mgr, err := getDeps(c).initializeHistoryManager(c)
	if err != nil {
		return invariant.ManagerFixResult{}, err
	}
	defer historyV2Mgr.Close()

	pr := persistence.NewPersistenceRetryer(
		execManager,
		historyV2Mgr,
		common.CreatePersistenceRetryPolicy(),
	)

	var ivs []invariant.Invariant

	for _, fn := range invariants {
		ivs = append(ivs, fn(pr, cache.NewNoOpDomainCache()))
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return invariant.ManagerFixResult{}, err
	}
	invariantManager, err := getDeps(c).initializeInvariantManager(ivs)
	if err != nil {
		return invariant.ManagerFixResult{}, err
	}

	return invariantManager.RunFixes(ctx, execution.Execution.(entity.Entity)), nil
}
