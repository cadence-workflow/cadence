package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func generateCadenceCommand(query, domain, env, proxyRegion string) (string, error) {

	query = strings.ToLower(query)
	baseCmd := fmt.Sprintf("cadence --env %s --proxy_region %s --domain %s ", env, proxyRegion, domain)

	if strings.Contains(query, "workflow") || strings.Contains(query, "workflows") {
		return generateWorkflowCommand(query, baseCmd), nil
	}
	return "", errors.New("unsupported query")
}

func generateWorkflowCommand(query, baseCmd string) string {

	// Show + Describe + History of Workflow Execution
	if strings.Contains(query, "show") || strings.Contains(query, "describe") || strings.Contains(query, "history") {
		return baseCmd + " workflow show --wid <workflow-id>"
	}
	// Start Workflow
	if strings.Contains(query, "start") || strings.Contains(query, "create") {
		return baseCmd + " workflow start --tl <tasklist-name> --wt <workflow-type> --input <input-json> --et <execution-timeout>"
	}
	// Terminate Workflow
	if strings.Contains(query, "terminate") || strings.Contains(query, "stop") {
		return baseCmd + " workflow terminate --wid <workflow-id> --reason <termination-reason>"
	}

	// TODO
	if strings.Contains(query, "list") || strings.Contains(query, "from") || strings.Contains(query, "past") {
		queryFilter := buildWorkflowQueryFilter(query)
		return baseCmd + " workflow list --query \" " + queryFilter + "\""
	}
	if strings.Contains(query, "count") {
		queryFilter := buildWorkflowQueryFilter(query)
		return baseCmd + " workflow count --query \" " + queryFilter + "\""
	}

	// Default to list all workflows
	return baseCmd + " workflow list --type <workflow-type>"
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
		filters = append(filters, "CloseStatus = 'FAILED'")
	}

	if strings.Contains(query, "completed") || strings.Contains(query, "success") {
		filters = append(filters, "CloseStatus = 'COMPLETED'")
	}

	if strings.Contains(query, "running") {
		filters = append(filters, "CloseStatus = 'RUNNING'")
	}

	if strings.Contains(query, "terminated") {
		filters = append(filters, "CloseStatus = 'TERMINATED'")
	}
	if strings.Contains(query, "timedout") || strings.Contains(query, "timeout") {
		filters = append(filters, "CloseStatus = 'TIMEOUT'")
	}

	// Check for workflow type filters
	if strings.Contains(query, "workflow type") || strings.Contains(query, "type") {
		filters = append(filters, "WorkflowType = '<workflow-type>'")
	}

	// Check for workflow ID filters
	if strings.Contains(query, "workflow id") || strings.Contains(query, "id") {
		filters = append(filters, "WorkflowID = '<workflow-id>'")
	}

	// Combine filters
	if len(filters) == 0 {
		return ""
	}

	return strings.Join(filters, " AND ")
}

func buildTimeFilter(query string) string {
	// Extract number of days/weeks/months
	days := extractDays(query)
	if days == 0 {
		return ""
	}

	// Calculate start time
	startTime := time.Now().AddDate(0, 0, -days)
	endTime := time.Now()

	// Format dates as ISO 8601
	startStr := startTime.Format("2006-01-02T15:04:05Z")
	endStr := endTime.Format("2006-01-02T15:04:05Z")

	return fmt.Sprintf("CloseTime >= '%s' AND CloseTime <= '%s'", startStr, endStr)
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
