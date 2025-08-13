package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Main function to generate Cadence command
func generateCadenceCommand(query, domain, address string) (string, error) {
	query = strings.ToLower(query)
	address = strings.ToLower(address)

	baseCmd := fmt.Sprintf("docker run -t --rm --network host ubercadence/cli:master --transport grpc --address %s --do %s", address, domain)

	if strings.Contains(query, "workflow") || strings.Contains(query, "workflows") {
		return generateWorkflowCommand(query, baseCmd)
	}

	if strings.Contains(query, "domain") {
		return generateDomainCommand(query, baseCmd)
	}

	return "", errors.New("unsupported query type")
}

func generateWorkflowCommand(query, baseCmd string) (string, error) {

	// Show/History of Workflow Execution
	if strings.Contains(query, "show") || strings.Contains(query, "history") {
		if strings.Contains(query, "workflow id") || strings.Contains(query, "id") {
			workflowID := extractWorkflowID(query)
			return fmt.Sprintf(baseCmd, "wf show --wid %s", workflowID), nil
		}
		return "Please provide a workflow ID to show workflow details", nil
	}

	// Describe Workflow
	if strings.Contains(query, "describe") {
		if strings.Contains(query, "workflow id") || strings.Contains(query, "id") {
			workflowID := extractWorkflowID(query)
			return fmt.Sprintf(baseCmd, "wf describe -w %s", workflowID), nil
		}
		return "Please provide a workflow ID to describe workflow details", nil
	}

	// Start Workflow
	if strings.Contains(query, "start") || strings.Contains(query, "create") {
		// Add search attributes if provided
		if strings.Contains(query, "search") || strings.Contains(query, "custom") {
			return fmt.Sprintf(baseCmd, "wf start --tl <tasklist-name> --wt <workflow-type> --et <execution-timeout> --i <input-json> -search_attr_key CustomIntField | CustomKeywordField | CustomStringField | CustomBoolField | CustomDatetimeField -search_attr_value <value> | keyword | <search-attribute> | true | 2019-06-07T16:16:36-08:00"), nil
		}
		return fmt.Sprintf(baseCmd, "wf start --tl <tasklist-name> --wt <workflow-type> --et <execution-timeout> -i <input-json>"), nil
	}

	// Run Workflow
	if strings.Contains(query, "run") {
		return fmt.Sprintf(baseCmd, "wf run --tl <tasklist-name> --wt <workflow-type> --et <execution-timeout> -i <input-json>"), nil
	}

	// Signal Workflow
	if strings.Contains(query, "signal") {
		if strings.Contains(query, "workflow id") || strings.Contains(query, "id") && strings.Contains(query, "run id") || strings.Contains(query, "run") {
			workflowID := extractWorkflowID(query)
			runID := extractRunID(query)
			return fmt.Sprintf(baseCmd, "wf signal -w %s -r %s -n <signal-name> -i <input-json>", workflowID, runID), nil
		}
		return "Please provide a workflow/run ID to signal workflow", nil
	}

	// Cancel Workflow
	if strings.Contains(query, "cancel") {
		if strings.Contains(query, "workflow id") || strings.Contains(query, "id") && strings.Contains(query, "run id") || strings.Contains(query, "run") {
			workflowID := extractWorkflowID(query)
			runID := extractRunID(query)
			return fmt.Sprintf(baseCmd, "wf cancel -w %s -r %s", workflowID, runID), nil
		}
		return "Please provide a workflow/run ID to cancel workflow", nil
	}

	// Terminate Workflow
	if strings.Contains(query, "terminate") || strings.Contains(query, "stop") {
		if strings.Contains(query, "workflow id") || strings.Contains(query, "id") && strings.Contains(query, "run id") || strings.Contains(query, "run") {
			workflowID := extractWorkflowID(query)
			runID := extractRunID(query)
			return fmt.Sprintf(baseCmd, "wf terminate -w %s -r %s --reason <reason>", workflowID, runID), nil
		}
		return "Please provide a workflow/run ID to terminate workflow", nil
	}

	// List Workflows with filters
	if strings.Contains(query, "list") || strings.Contains(query, "from") || strings.Contains(query, "past") {
		queryFilter := buildWorkflowQueryFilter(query)
		if queryFilter != "" {
			return fmt.Sprintf(baseCmd, "wf list -q %s", queryFilter), nil
		}
		return fmt.Sprintf(baseCmd, "wf list -m"), nil
	}

	// Count Workflows
	if strings.Contains(query, "count") {
		queryFilter := buildWorkflowQueryFilter(query)
		return fmt.Sprintf(baseCmd, "wf count -q %s", queryFilter), nil
	}

	// Query Workflows using Stack Trace
	if strings.Contains(query, "stack trace") || strings.Contains(query, "stack") {
		if strings.Contains(query, "workflow id") || strings.Contains(query, "id") && strings.Contains(query, "run id") || strings.Contains(query, "run") {
			workflowID := extractWorkflowID(query)
			runID := extractRunID(query)
			return fmt.Sprintf(baseCmd, "wf stack -w %s -r %s", workflowID, runID), nil
		}
		return "Please provide a workflow ID/runID to query stack trace", nil
	}

	// Reset Workflow
	if strings.Contains(query, "reset") {
		if strings.Contains(query, "workflow id") || strings.Contains(query, "id") && strings.Contains(query, "run id") || strings.Contains(query, "run") {
			workflowID := extractWorkflowID(query)
			runID := extractRunID(query)
			return fmt.Sprintf(baseCmd, "wf reset -w %s -r %s --reset_type <reset-type> --reason <reason>", workflowID, runID), nil
		}
		return "Please provide a workflow ID to reset workflow", nil
	}

	return "", errors.New("unsupported query type")
}

func generateDomainCommand(query, baseCmd string) (string, error) {
	if strings.Contains(query, "describe") || strings.Contains(query, "show") {
		return fmt.Sprintf(baseCmd, "domain describe"), nil
	} else if strings.Contains(query, "list") {
		return fmt.Sprintf(baseCmd, "domain list"), nil
	}
	return "", errors.New("unsupported query type")
}

func extractWorkflowID(query string) string {
	re := regexp.MustCompile(`(?:workflow\s+)?id\s+([a-zA-Z0-9\-_]+)`)
	matches := re.FindStringSubmatch(query)
	if len(matches) > 1 {
		return matches[1]
	}
	return "<workflow-id>"
}

func extractRunID(query string) string {
	re := regexp.MustCompile(`(?:run\s+)?id\s+([a-zA-Z0-9\-_]+)`)
	matches := re.FindStringSubmatch(query)
	if len(matches) > 1 {
		return matches[1]
	}
	return "<run-id>"
}

func buildWorkflowQueryFilter(query string) string {
	var filters []string

	// Check for time-based filters
	if strings.Contains(query, "past") || strings.Contains(query, "last") {
		timeFilter := buildTimeFilter(query)
		if timeFilter != "" {
			filters = append(filters, timeFilter)
		}
	}

	// Check for status filters
	if strings.Contains(query, "failed") || strings.Contains(query, "failure") {
		filters = append(filters, "CloseStatus=\"failed\"")
	}

	if strings.Contains(query, "completed") || strings.Contains(query, "success") {
		filters = append(filters, "CloseStatus=\"completed\"")
	}

	if strings.Contains(query, "canceled") {
		filters = append(filters, "CloseStatus=\"canceled\"")
	}

	if strings.Contains(query, "terminated") {
		filters = append(filters, "CloseStatus=\"terminated\"")
	}

	if strings.Contains(query, "timed out") || strings.Contains(query, "timeout") {
		filters = append(filters, "CloseStatus=\"timed_out\"")
	}

	if strings.Contains(query, "continued as new") {
		filters = append(filters, "CloseStatus=\"continued_as_new\"")
	}

	// Check for workflow type filters
	if strings.Contains(query, "workflow type") || strings.Contains(query, "type") {
		filters = append(filters, "WorkflowType = \"<workflow-type>\"")
	}

	// Check for workflow ID filters
	if strings.Contains(query, "workflow id") || strings.Contains(query, "id") {
		filters = append(filters, "WorkflowID = \"<workflow-id>\"")
	}

	// Check for run ID filters
	if strings.Contains(query, "run id") || strings.Contains(query, "id") {
		filters = append(filters, "RunID = \"<run-id>\"")
	}

	// Check for search attribute filters
	if strings.Contains(query, "search") || strings.Contains(query, "custom") {
		searchFilters := buildSearchAttributeFilters(query)
		if len(searchFilters) > 0 {
			filters = append(filters, searchFilters...)
		}
	}

	// Combine filters
	if len(filters) == 0 {
		return ""
	}

	return strings.Join(filters, " AND ")
}

func buildSearchAttributeFilters(query string) []string {
	var filters []string

	// Check for common search attribute patterns
	if strings.Contains(query, "customint") || strings.Contains(query, "int field") {
		filters = append(filters, "CustomIntField >= 0")
	}

	if strings.Contains(query, "customkeyword") || strings.Contains(query, "keyword field") {
		filters = append(filters, "CustomKeywordField = \"<keyword-value>\"")
	}

	if strings.Contains(query, "customstring") || strings.Contains(query, "string field") {
		filters = append(filters, "CustomStringField = \"<string-value>\"")
	}

	if strings.Contains(query, "custombool") || strings.Contains(query, "bool field") {
		filters = append(filters, "CustomBoolField = true")
	}

	if strings.Contains(query, "customdatetime") || strings.Contains(query, "datetime field") {
		filters = append(filters, "CustomDatetimeField > \"<datetime-value>\"")
	}

	return filters
}

func buildTimeFilter(query string) string {
	// Extract number of days/weeks/months
	days := extractDays(query)
	if days == 0 {
		return ""
	}

	// Check for reference date like "today is august 11"
	referenceDate := extractReferenceDate(query)
	var baseTime time.Time
	if !referenceDate.IsZero() {
		baseTime = referenceDate
	} else {
		baseTime = time.Now()
	}

	// Calculate start time from reference date
	startTime := baseTime.AddDate(0, 0, -days)
	endTime := baseTime

	// Format dates as ISO 8601
	startStr := startTime.Format("2006-01-02T15:04:05Z")
	endStr := endTime.Format("2006-01-02T15:04:05Z")

	return fmt.Sprintf("CloseTime between \"%s\" and \"%s\"", startStr, endStr)
}

func extractReferenceDate(query string) time.Time {
	// Look for patterns like "today is august 11", "today is 2025-08-11"
	patterns := []string{
		`today is (january|february|march|april|may|june|july|august|september|october|november|december)\s+(\d+)`,
		`today is (\d{4})-(\d{1,2})-(\d{1,2})`,
		`reference date is (january|february|march|april|may|june|july|august|september|october|november|december)\s+(\d+)`,
		`reference date is (\d{4})-(\d{1,2})-(\d{1,2})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(query)
		if len(matches) > 1 {
			if len(matches) == 3 {
				// Month name + day format
				monthStr := matches[1]
				dayStr := matches[2]

				// Map month names to numbers
				monthMap := map[string]int{
					"january": 1, "february": 2, "march": 3, "april": 4,
					"may": 5, "june": 6, "july": 7, "august": 8,
					"september": 9, "october": 10, "november": 11, "december": 12,
				}

				if month, ok := monthMap[strings.ToLower(monthStr)]; ok {
					if day, err := strconv.Atoi(dayStr); err == nil {
						// Use current year
						year := time.Now().Year()
						return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
					}
				}
			} else if len(matches) == 4 {
				// YYYY-MM-DD format
				yearStr := matches[1]
				monthStr := matches[2]
				dayStr := matches[3]

				if year, err := strconv.Atoi(yearStr); err == nil {
					if month, err := strconv.Atoi(monthStr); err == nil {
						if day, err := strconv.Atoi(dayStr); err == nil {
							return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
						}
					}
				}
			}
		}
	}

	return time.Time{} // Return zero time if no reference date found
}

func extractDays(query string) int {
	// Look for patterns like "past 7 days", "last 3 days", "7 days ago"
	patterns := []string{
		`past (\d+) days?`,
		`last (\d+) days?`,
		`(\d+) days? ago`,
		`(\d+) days?`,
		`past (\d+) weeks?`,
		`last (\d+) weeks?`,
		`(\d+) weeks? ago`,
		`(\d+) weeks?`,
		`past (\d+) months?`,
		`last (\d+) months?`,
		`(\d+) months? ago`,
		`(\d+) months?`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(query)
		if len(matches) > 1 {
			value, err := strconv.Atoi(matches[1])
			if err == nil {
				// Convert weeks and months to days
				if strings.Contains(pattern, "weeks") {
					return value * 7
				}
				if strings.Contains(pattern, "months") {
					return value * 30
				}
				return value
			}
		}
	}

	return 0
}
