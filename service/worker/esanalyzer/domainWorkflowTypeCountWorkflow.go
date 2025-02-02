// Copyright (c) 2021 Uber Technologies, Inc.
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

package esanalyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/cadence"
	"go.uber.org/cadence/activity"
	cclient "go.uber.org/cadence/client"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/pinot"
)

const (
	workflowTypeCountMetrics = "workflow_type_count"

	// workflow constants
	domainWFTypeCountWorkflowID                = "cadence-sys-tl-esanalyzer-domain-wf-type-count"
	domainWFTypeCountWorkflowTypeName          = "cadence-sys-es-analyzer-domain-wf-type-count-workflow"
	emitDomainWorkflowTypeCountMetricsActivity = "cadence-sys-es-analyzer-emit-domain-workflow-type-count-metrics"
)

type (
	DomainWorkflowTypeCount struct {
		WorkflowTypes []EsAggregateCount `json:"buckets"`
	}
)

var (
	workflowActivityOptions = workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    5 * time.Minute,
		RetryPolicy: &cadence.RetryPolicy{
			InitialInterval:    10 * time.Second,
			BackoffCoefficient: 1.7,
			MaximumInterval:    5 * time.Minute,
			ExpirationInterval: 10 * time.Minute,
		},
	}

	domainWfTypeCountStartOptions = cclient.StartWorkflowOptions{
		ID:                           domainWFTypeCountWorkflowID,
		TaskList:                     taskListName,
		ExecutionStartToCloseTimeout: 5 * time.Minute,
		CronSchedule:                 "*/5 * * * *",
	}
)

func initDomainWorkflowTypeCountWorkflow(a *Analyzer) {
	w := Workflow{analyzer: a}
	workflow.RegisterWithOptions(w.emitWorkflowTypeCount, workflow.RegisterOptions{Name: domainWFTypeCountWorkflowTypeName})
	activity.RegisterWithOptions(
		w.emitWorkflowTypeCountMetrics,
		activity.RegisterOptions{Name: emitDomainWorkflowTypeCountMetricsActivity},
	)
}

// emitWorkflowTypeCount queries ElasticSearch for workflow count per type and emit metrics
func (w *Workflow) emitWorkflowTypeCount(ctx workflow.Context) error {
	if w.analyzer.config.ESAnalyzerPause() {
		logger := workflow.GetLogger(ctx)
		logger.Info("Skipping ESAnalyzer execution cycle since it was paused")
		return nil
	}
	var err error
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, workflowActivityOptions),
		emitDomainWorkflowTypeCountMetricsActivity,
	).Get(ctx, &err)
	if err != nil {
		return err
	}
	return nil
}

// get open workflows count per workflow type for specified domain
func (w *Workflow) getDomainWorkflowTypeCountQuery(domainName string) (string, error) {
	domain, err := w.analyzer.domainCache.GetDomain(domainName)
	if err != nil {
		return "", err
	}
	// exclude uninitialized workflow executions by checking whether record has start time field
	return fmt.Sprintf(`
{
    "aggs" : {
        "wftypes" : {
            "terms" : { "field" : "WorkflowType"}
        }
    },
    "query": {
        "bool": {
            "must_not": {
                "exists": {
                    "field": "CloseTime"
                }
            },
            "must": [
                {
                    "match" : {
                        "DomainID" : "%s"
                    }
                },
				{
					"exists": {
						"field": "StartTime" 
					}
				}
            ]
        }
    },
    "size": 0
}
    `, domain.GetInfo().ID), nil
}

// emitWorkflowTypeCountMetrics is an activity that emits the running workflow type counts of a domain
// it will switch between ES and Pinot based on the readMode
func (w *Workflow) emitWorkflowTypeCountMetrics(ctx context.Context) error {
	logger := activity.GetLogger(ctx)
	var workflowMetricDomainNames []string
	workflowMetricDomains := w.analyzer.config.ESAnalyzerWorkflowTypeDomains()
	if len(workflowMetricDomains) > 0 {
		err := json.Unmarshal([]byte(workflowMetricDomains), &workflowMetricDomainNames)
		if err != nil {
			return err
		}
		var failedDomains []string
		for _, domainName := range workflowMetricDomainNames {
			switch w.analyzer.readMode {
			case ES:
				err = w.emitWorkflowTypeCountMetricsES(ctx, domainName, logger)
			case Pinot:
				err = w.emitWorkflowTypeCountMetricsPinot(domainName, logger)
			default:
				err = w.emitWorkflowTypeCountMetricsES(ctx, domainName, logger)
			}
			if err != nil {
				logger.Error(fmt.Sprintf("failed to emit workflow type metrics for domain %s", domainName), zap.Error(err))
				failedDomains = append(failedDomains, domainName)
			}
		}
		if len(failedDomains) == len(workflowMetricDomainNames) {
			return fmt.Errorf("failed to emit workflow type metrics for all domains")
		}
	}
	return nil
}

func (w *Workflow) getDomainWorkflowTypeCountPinotQuery(domainName string) (string, error) {
	domain, err := w.analyzer.domainCache.GetDomain(domainName)
	if err != nil {
		return "", err
	}
	// exclude uninitialized workflow executions by checking whether record has start time field
	// there's a "LIMIT 10" because in ES, Aggr clause by default returns the top 10 results
	return fmt.Sprintf(`
SELECT WorkflowType, COUNT(*) AS count
FROM %s
WHERE DomainID = '%s'
AND CloseStatus = -1
AND StartTime > 0
GROUP BY WorkflowType
ORDER BY count
LIMIT 10
    `, w.analyzer.pinotTableName, domain.GetInfo().ID), nil
}

func (w *Workflow) emitWorkflowTypeCountMetricsPinot(domainName string, logger *zap.Logger) error {
	wfTypeCountPinotQuery, err := w.getDomainWorkflowTypeCountPinotQuery(domainName)
	if err != nil {
		logger.Error("Failed to get Pinot query to find domain workflow type Info",
			zap.Error(err),
			zap.String("DomainName", domainName),
		)
		return err
	}
	response, err := w.analyzer.pinotClient.SearchAggr(&pinot.SearchRequest{Query: wfTypeCountPinotQuery})
	if err != nil {
		logger.Error("Failed to query Pinot to find workflow type count Info",
			zap.Error(err),
			zap.String("VisibilityQuery", wfTypeCountPinotQuery),
			zap.String("DomainName", domainName),
		)
		return err
	}
	foundAggregation := len(response) > 0

	if !foundAggregation {
		logger.Error("Pinot error: aggregation failed.",
			zap.Error(err),
			zap.String("Aggregation", fmt.Sprintf("%v", response)),
			zap.String("DomainName", domainName),
			zap.String("VisibilityQuery", wfTypeCountPinotQuery),
		)
		return fmt.Errorf("aggregation failed for domain in Pinot: %s", domainName)
	}
	var domainWorkflowTypeCount DomainWorkflowTypeCount
	for _, row := range response {
		workflowType := row[0].(string)

		// even though the count is a int, it is returned as a float64, need to pay attention to this
		workflowCount, ok := row[1].(float64)
		if !ok {
			logger.Error("Error parsing workflow count",
				zap.Error(err),
				zap.String("WorkflowType", workflowType),
				zap.String("DomainName", domainName),
				zap.Float64("WorkflowCount", workflowCount),
				zap.String("WorkflowCountType", fmt.Sprintf("%T", row[1])),
				zap.String("raw data", fmt.Sprintf("%#v", response)),
			)
			return fmt.Errorf("error parsing workflow count for workflow type %s", workflowType)
		}
		domainWorkflowTypeCount.WorkflowTypes = append(domainWorkflowTypeCount.WorkflowTypes, EsAggregateCount{
			AggregateKey:   workflowType,
			AggregateCount: int64(workflowCount),
		})
	}
	for _, workflowType := range domainWorkflowTypeCount.WorkflowTypes {
		w.analyzer.tallyScope.Tagged(
			map[string]string{domainTag: domainName, workflowTypeTag: workflowType.AggregateKey},
		).Gauge(workflowTypeCountMetrics).Update(float64(workflowType.AggregateCount))
	}
	return nil
}

func (w *Workflow) emitWorkflowTypeCountMetricsES(ctx context.Context, domainName string, logger *zap.Logger) error {
	wfTypeCountEsQuery, err := w.getDomainWorkflowTypeCountQuery(domainName)
	if err != nil {
		logger.Error("Failed to get ElasticSearch query to find domain workflow type Info",
			zap.Error(err),
			zap.String("DomainName", domainName),
		)
		return err
	}
	response, err := w.analyzer.esClient.SearchRaw(ctx, w.analyzer.visibilityIndexName, wfTypeCountEsQuery)
	if err != nil {
		logger.Error("Failed to query ElasticSearch to find workflow type count Info",
			zap.Error(err),
			zap.String("VisibilityQuery", wfTypeCountEsQuery),
			zap.String("DomainName", domainName),
		)
		return err
	}
	agg, foundAggregation := response.Aggregations[workflowTypesAggKey]

	if !foundAggregation {
		logger.Error("ElasticSearch error: aggregation failed.",
			zap.Error(err),
			zap.String("Aggregation", string(agg)),
			zap.String("DomainName", domainName),
			zap.String("VisibilityQuery", wfTypeCountEsQuery),
		)
		return fmt.Errorf("aggregation failed for domain in ES: %s", domainName)
	}
	var domainWorkflowTypeCount DomainWorkflowTypeCount
	err = json.Unmarshal(agg, &domainWorkflowTypeCount)
	if err != nil {
		logger.Error("ElasticSearch error parsing aggregation.",
			zap.Error(err),
			zap.String("Aggregation", string(agg)),
			zap.String("DomainName", domainName),
			zap.String("VisibilityQuery", wfTypeCountEsQuery),
		)
		return err
	}
	for _, workflowType := range domainWorkflowTypeCount.WorkflowTypes {
		w.analyzer.tallyScope.Tagged(
			map[string]string{domainTag: domainName, workflowTypeTag: workflowType.AggregateKey},
		).Gauge(workflowTypeCountMetrics).Update(float64(workflowType.AggregateCount))
	}
	return nil
}
