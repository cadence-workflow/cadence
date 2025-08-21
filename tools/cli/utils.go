// Copyright (c) 2017-2020 Uber Technologies Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/golang-jwt/jwt/v5"
	"github.com/urfave/cli/v2"
	"github.com/valyala/fastjson"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/authorization"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/pagination"
	"github.com/uber/cadence/common/types"
)

// JSONHistorySerializer is used to encode history event in JSON
type JSONHistorySerializer struct{}

// Serialize serializes history.
func (j *JSONHistorySerializer) Serialize(h *types.History) ([]byte, error) {
	return json.Marshal(h.Events)
}

// Deserialize deserializes history
func (j *JSONHistorySerializer) Deserialize(data []byte) (*types.History, error) {
	var events []*types.HistoryEvent
	err := json.Unmarshal(data, &events)
	if err != nil {
		return nil, err
	}
	return &types.History{Events: events}, nil
}

// GetHistory helper method to iterate over all pages and return complete list of history events
func GetHistory(ctx context.Context, workflowClient frontend.Client, domain, workflowID, runID string, consistencyLevel *types.QueryConsistencyLevel) (*types.History, error) {
	events := []*types.HistoryEvent{}
	iterator, err := GetWorkflowHistoryIterator(ctx, workflowClient, domain, workflowID, runID, false, types.HistoryEventFilterTypeAllEvent.Ptr(), consistencyLevel)
	for iterator.HasNext() {
		entity, err := iterator.Next()
		if err != nil {
			return nil, err
		}
		events = append(events, entity.(*types.HistoryEvent))
	}
	history := &types.History{}
	history.Events = events
	return history, err
}

// GetWorkflowHistoryIterator returns a HistoryEvent iterator
func GetWorkflowHistoryIterator(
	ctx context.Context,
	workflowClient frontend.Client,
	domain,
	workflowID,
	runID string,
	isLongPoll bool,
	filterType *types.HistoryEventFilterType,
	consistencyLevel *types.QueryConsistencyLevel,
) (pagination.Iterator, error) {
	paginate := func(ctx context.Context, pageToken pagination.PageToken) (pagination.Page, error) {
		tcCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()

		var nextPageToken []byte
		if pageToken != nil {
			nextPageToken, _ = pageToken.([]byte)
		}
		request := &types.GetWorkflowExecutionHistoryRequest{
			Domain: domain,
			Execution: &types.WorkflowExecution{
				WorkflowID: workflowID,
				RunID:      runID,
			},
			WaitForNewEvent:        isLongPoll,
			HistoryEventFilterType: filterType,
			NextPageToken:          nextPageToken,
			SkipArchival:           isLongPoll,
			QueryConsistencyLevel:  consistencyLevel,
		}

		var resp *types.GetWorkflowExecutionHistoryResponse
		var err error
	Loop:
		for {
			resp, err = workflowClient.GetWorkflowExecutionHistory(tcCtx, request)
			if err != nil {
				return pagination.Page{}, err
			}

			if isLongPoll && len(resp.History.Events) == 0 && len(resp.NextPageToken) != 0 {
				request.NextPageToken = resp.NextPageToken
				continue Loop
			}
			break Loop
		}
		entities := make([]pagination.Entity, len(resp.History.Events))
		for i, e := range resp.History.Events {
			entities[i] = e
		}
		var nextToken interface{} = resp.NextPageToken
		if len(resp.NextPageToken) == 0 {
			nextToken = nil
		}
		page := pagination.Page{
			CurrentToken: pageToken,
			NextToken:    nextToken,
			Entities:     entities,
		}
		return page, err
	}
	return pagination.NewIterator(ctx, nil, paginate), nil
}

// HistoryEventToString convert HistoryEvent to string
func HistoryEventToString(e *types.HistoryEvent, printFully bool, maxFieldLength int) string {
	data := getEventAttributes(e)
	return anyToString(data, printFully, maxFieldLength)
}

func anyToString(d interface{}, printFully bool, maxFieldLength int) string {
	// fields related to schedule are of time.Time type, and we shouldn't dive
	// into it with reflection - it's fields are private.
	tm, ok := d.(time.Time)
	if ok {
		return trimText(tm.String(), maxFieldLength)
	}

	v := reflect.ValueOf(d)
	switch v.Kind() {
	case reflect.Ptr:
		return anyToString(v.Elem().Interface(), printFully, maxFieldLength)
	case reflect.Struct:
		var buf bytes.Buffer
		t := reflect.TypeOf(d)
		buf.WriteString("{")
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			if f.Kind() == reflect.Invalid {
				continue
			}
			fieldValue := valueToString(f, printFully, maxFieldLength)
			if len(fieldValue) == 0 {
				continue
			}
			if buf.Len() > 1 {
				buf.WriteString(", ")
			}
			fieldName := t.Field(i).Name
			if !isAttributeName(fieldName) {
				if !printFully {
					fieldValue = trimTextAndBreakWords(fieldValue, maxFieldLength)
				} else if maxFieldLength != 0 { // for command run workflow and observe history
					fieldValue = trimText(fieldValue, maxFieldLength)
				}
			}
			if fieldName == "Reason" || fieldName == "Details" || fieldName == "Cause" {
				buf.WriteString(fmt.Sprintf("%s:%s", color.RedString(fieldName), color.MagentaString(fieldValue)))
			} else {
				buf.WriteString(fmt.Sprintf("%s:%s", fieldName, fieldValue))
			}
		}
		buf.WriteString("}")
		return buf.String()
	default:
		return fmt.Sprint(d)
	}
}

func valueToString(v reflect.Value, printFully bool, maxFieldLength int) string {
	switch v.Kind() {
	case reflect.Ptr:
		return valueToString(v.Elem(), printFully, maxFieldLength)
	case reflect.Struct:
		return anyToString(v.Interface(), printFully, maxFieldLength)
	case reflect.Invalid:
		return ""
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			n := string(v.Bytes())
			if n != "" && n[len(n)-1] == '\n' {
				return fmt.Sprintf("[%v]", n[:len(n)-1])
			}
			return fmt.Sprintf("[%v]", n)
		}
		return fmt.Sprintf("[len=%d]", v.Len())
	case reflect.Map:
		str := "map{"
		for i, key := range v.MapKeys() {
			str += key.String() + ":"
			val := v.MapIndex(key)
			switch val.Interface().(type) {
			case []byte:
				str += string(val.Interface().([]byte))
			default:
				str += val.String()
			}
			if i != len(v.MapKeys())-1 {
				str += ", "
			}
		}
		str += "}"
		return str
	default:
		return fmt.Sprint(v.Interface())
	}
}

// limit the maximum length for each field
func trimText(input string, maxFieldLength int) string {
	if len(input) > maxFieldLength {
		input = fmt.Sprintf("%s ... %s", input[:maxFieldLength/2], input[(len(input)-maxFieldLength/2):])
	}
	return input
}

// limit the maximum length for each field, and break long words for table item correctly wrap words
func trimTextAndBreakWords(input string, maxFieldLength int) string {
	input = trimText(input, maxFieldLength)
	return breakLongWords(input, maxWordLength)
}

// long words will make output in table cell looks bad,
// break long text "ltltltltllt..." to "ltlt ltlt lt..." will make use of table autowrap so that output is pretty.
func breakLongWords(input string, maxWordLength int) string {
	if len(input) <= maxWordLength {
		return input
	}

	cnt := 0
	for i := 0; i < len(input); i++ {
		if cnt == maxWordLength {
			cnt = 0
			input = input[:i] + " " + input[i:]
			continue
		}
		cnt++
		if input[i] == ' ' {
			cnt = 0
		}
	}
	return input
}

// ColorEvent takes an event and return string with color
// Event with color mapping rules:
//
//	Failed - red
//	Timeout - yellow
//	Canceled - magenta
//	Completed - green
//	Started - blue
//	Others - default (white/black)
func ColorEvent(e *types.HistoryEvent) string {
	f := EventColorFunction(*e.EventType)
	return f(e.EventType.String())
}

func EventColorFunction(eventType types.EventType) func(format string, a ...interface{}) string {
	var colorFunc func(format string, a ...interface{}) string
	noColorFunc := func(format string, a ...interface{}) string {
		return format
	}
	switch eventType {
	case types.EventTypeWorkflowExecutionStarted,
		types.EventTypeChildWorkflowExecutionStarted:
		colorFunc = color.BlueString

	case types.EventTypeWorkflowExecutionCompleted,
		types.EventTypeChildWorkflowExecutionCompleted:
		colorFunc = color.GreenString

	case types.EventTypeWorkflowExecutionFailed,
		types.EventTypeRequestCancelActivityTaskFailed,
		types.EventTypeCancelTimerFailed,
		types.EventTypeStartChildWorkflowExecutionFailed,
		types.EventTypeChildWorkflowExecutionFailed,
		types.EventTypeRequestCancelExternalWorkflowExecutionFailed,
		types.EventTypeSignalExternalWorkflowExecutionFailed,
		types.EventTypeActivityTaskFailed:
		colorFunc = color.RedString

	case types.EventTypeWorkflowExecutionTimedOut,
		types.EventTypeActivityTaskTimedOut,
		types.EventTypeWorkflowExecutionCanceled,
		types.EventTypeChildWorkflowExecutionTimedOut,
		types.EventTypeDecisionTaskTimedOut:
		colorFunc = color.YellowString

	case types.EventTypeChildWorkflowExecutionCanceled:
		colorFunc = color.MagentaString

	default:
		colorFunc = noColorFunc
	}

	return colorFunc
}

func getEventAttributes(e *types.HistoryEvent) interface{} {
	var data interface{}
	switch e.GetEventType() {
	case types.EventTypeWorkflowExecutionStarted:
		data = e.WorkflowExecutionStartedEventAttributes

	case types.EventTypeWorkflowExecutionCompleted:
		data = e.WorkflowExecutionCompletedEventAttributes

	case types.EventTypeWorkflowExecutionFailed:
		data = e.WorkflowExecutionFailedEventAttributes

	case types.EventTypeWorkflowExecutionTimedOut:
		data = e.WorkflowExecutionTimedOutEventAttributes

	case types.EventTypeDecisionTaskScheduled:
		data = e.DecisionTaskScheduledEventAttributes

	case types.EventTypeDecisionTaskStarted:
		data = e.DecisionTaskStartedEventAttributes

	case types.EventTypeDecisionTaskCompleted:
		data = e.DecisionTaskCompletedEventAttributes

	case types.EventTypeDecisionTaskTimedOut:
		data = e.DecisionTaskTimedOutEventAttributes

	case types.EventTypeActivityTaskScheduled:
		data = e.ActivityTaskScheduledEventAttributes

	case types.EventTypeActivityTaskStarted:
		data = e.ActivityTaskStartedEventAttributes

	case types.EventTypeActivityTaskCompleted:
		data = e.ActivityTaskCompletedEventAttributes

	case types.EventTypeActivityTaskFailed:
		data = e.ActivityTaskFailedEventAttributes

	case types.EventTypeActivityTaskTimedOut:
		data = e.ActivityTaskTimedOutEventAttributes

	case types.EventTypeActivityTaskCancelRequested:
		data = e.ActivityTaskCancelRequestedEventAttributes

	case types.EventTypeRequestCancelActivityTaskFailed:
		data = e.RequestCancelActivityTaskFailedEventAttributes

	case types.EventTypeActivityTaskCanceled:
		data = e.ActivityTaskCanceledEventAttributes

	case types.EventTypeTimerStarted:
		data = e.TimerStartedEventAttributes

	case types.EventTypeTimerFired:
		data = e.TimerFiredEventAttributes

	case types.EventTypeCancelTimerFailed:
		data = e.CancelTimerFailedEventAttributes

	case types.EventTypeTimerCanceled:
		data = e.TimerCanceledEventAttributes

	case types.EventTypeWorkflowExecutionCancelRequested:
		data = e.WorkflowExecutionCancelRequestedEventAttributes

	case types.EventTypeWorkflowExecutionCanceled:
		data = e.WorkflowExecutionCanceledEventAttributes

	case types.EventTypeRequestCancelExternalWorkflowExecutionInitiated:
		data = e.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes

	case types.EventTypeRequestCancelExternalWorkflowExecutionFailed:
		data = e.RequestCancelExternalWorkflowExecutionFailedEventAttributes

	case types.EventTypeExternalWorkflowExecutionCancelRequested:
		data = e.ExternalWorkflowExecutionCancelRequestedEventAttributes

	case types.EventTypeMarkerRecorded:
		data = e.MarkerRecordedEventAttributes

	case types.EventTypeWorkflowExecutionSignaled:
		data = e.WorkflowExecutionSignaledEventAttributes

	case types.EventTypeWorkflowExecutionTerminated:
		data = e.WorkflowExecutionTerminatedEventAttributes

	case types.EventTypeWorkflowExecutionContinuedAsNew:
		data = e.WorkflowExecutionContinuedAsNewEventAttributes

	case types.EventTypeStartChildWorkflowExecutionInitiated:
		data = e.StartChildWorkflowExecutionInitiatedEventAttributes

	case types.EventTypeStartChildWorkflowExecutionFailed:
		data = e.StartChildWorkflowExecutionFailedEventAttributes

	case types.EventTypeChildWorkflowExecutionStarted:
		data = e.ChildWorkflowExecutionStartedEventAttributes

	case types.EventTypeChildWorkflowExecutionCompleted:
		data = e.ChildWorkflowExecutionCompletedEventAttributes

	case types.EventTypeChildWorkflowExecutionFailed:
		data = e.ChildWorkflowExecutionFailedEventAttributes

	case types.EventTypeChildWorkflowExecutionCanceled:
		data = e.ChildWorkflowExecutionCanceledEventAttributes

	case types.EventTypeChildWorkflowExecutionTimedOut:
		data = e.ChildWorkflowExecutionTimedOutEventAttributes

	case types.EventTypeChildWorkflowExecutionTerminated:
		data = e.ChildWorkflowExecutionTerminatedEventAttributes

	case types.EventTypeSignalExternalWorkflowExecutionInitiated:
		data = e.SignalExternalWorkflowExecutionInitiatedEventAttributes

	case types.EventTypeSignalExternalWorkflowExecutionFailed:
		data = e.SignalExternalWorkflowExecutionFailedEventAttributes

	case types.EventTypeExternalWorkflowExecutionSignaled:
		data = e.ExternalWorkflowExecutionSignaledEventAttributes

	default:
		data = e
	}
	return data
}

func isAttributeName(name string) bool {
	for i := types.EventType(0); i < types.EventType(40); i++ {
		if name == i.String()+"EventAttributes" {
			return true
		}
	}
	return false
}

func getCurrentUserFromEnv() string {
	for _, n := range envKeysForUserName {
		if len(os.Getenv(n)) > 0 {
			return os.Getenv(n)
		}
	}
	return "unknown"
}

func prettyPrintJSONObject(writer io.Writer, o interface{}) {
	b, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		writer.Write([]byte(fmt.Sprintf("Error when try to print pretty: %v\n", err)))
		writer.Write([]byte(fmt.Sprintf("%+v\n", o)))
	}
	writer.Write(b)
	writer.Write([]byte("\n"))
}

func mapKeysToArray(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func intSliceToSet(s []int) map[int]struct{} {
	var ret = make(map[int]struct{}, len(s))
	for _, v := range s {
		ret[v] = struct{}{}
	}
	return ret
}

func printMessage(output io.Writer, msg string) {
	output.Write([]byte(fmt.Sprintf("%s %s\n", "cadence:", msg)))
}

func printError(output io.Writer, msg string, err error) {
	if err != nil {
		output.Write([]byte(fmt.Sprintf("%s %s\n%s %+v\n", colorRed("Error:"), msg, colorMagenta("Error Details:"), err)))
		if os.Getenv(showErrorStackEnv) != `` {
			fmt.Printf("Stack trace:\n")
			debug.PrintStack()
		} else {
			output.Write([]byte(fmt.Sprintf("('export %s=1' to see stack traces)\n", showErrorStackEnv)))
		}
	} else {
		output.Write([]byte(fmt.Sprintf("%s %s\n", colorRed("Error:"), msg)))
	}
}

func getWorkflowClient(c *cli.Context) (frontend.Client, error) {
	return getDeps(c).ServerFrontendClient(c)
}

// getOptionWithSerializer is a helper function that retrieves a non-required option using a serializer function.
// To support nil as a valid value, returns nil when the option is not set.
// Do not use this function with a pointer type (e.g getOptionWithSerializer[*int]).
// Returns a pointer to the parsed value when the option is serialized.
// Returns an error when the serializer function is unable to parse the option value.
func getOptionWithSerializer[T any](c *cli.Context, optionName string, serializer func(string) (T, error)) (*T, error) {
	if !c.IsSet(optionName) {
		return nil, nil
	}

	value, err := serializer(c.String(optionName))

	return &value, err
}

func getRequiredOption(c *cli.Context, optionName string) (string, error) {
	value := c.String(optionName)
	if len(value) == 0 {
		return "", fmt.Errorf("option %s is required", optionName)
	}
	return value, nil
}

func getRequiredInt64Option(c *cli.Context, optionName string) (int64, error) {
	if !c.IsSet(optionName) {
		return 0, fmt.Errorf("option %s is required", optionName)
	}
	return c.Int64(optionName), nil
}

func getRequiredIntOption(c *cli.Context, optionName string) (int, error) {
	if !c.IsSet(optionName) {
		return 0, fmt.Errorf("option %s is required", optionName)
	}
	return c.Int(optionName), nil
}

func timestampPtrToStringPtr(unixNanoPtr *int64, onlyTime bool) *string {
	if unixNanoPtr == nil {
		return nil
	}
	return common.StringPtr(timestampToString(*unixNanoPtr, onlyTime))
}

func timestampToString(unixNano int64, onlyTime bool) string {
	t := time.Unix(0, unixNano).UTC()
	var result string
	if onlyTime {
		result = t.Format(defaultTimeFormat)
	} else {
		result = t.Format(defaultDateTimeFormat)
	}
	return result
}

func parseTime(timeStr string, defaultValue int64) (int64, error) {
	if len(timeStr) == 0 {
		return defaultValue, nil
	}

	// try to parse
	parsedTime, err := time.Parse(defaultDateTimeFormat, timeStr)
	if err == nil {
		return parsedTime.UnixNano(), nil
	}

	// treat as raw time
	resultValue, err := strconv.ParseInt(timeStr, 10, 64)
	if err == nil {
		return resultValue, nil
	}

	// treat as time range format
	parsedTime, err = parseTimeRange(timeStr)
	if err != nil {
		return 0, fmt.Errorf("cannot parse time '%s', use UTC format '2006-01-02T15:04:05Z', "+
			"time range or raw UnixNano directly. see help for more details: %v", timeStr, err)
	}
	return parsedTime.UnixNano(), nil
}

// parseTimeRange parses a given time duration string (in format X<time-duration>) and
// returns parsed timestamp given that duration in the past from current time.
// All valid values must contain a number followed by a time-duration, from the following list (long form/short form):
// - second/s
// - minute/m
// - hour/h
// - day/d
// - week/w
// - month/M
// - year/y
// For example, possible input values, and their result:
// - "3d" or "3day" --> three days --> time.Now().Add(-3 * 24 * time.Hour)
// - "2m" or "2minute" --> two minutes --> time.Now().Add(-2 * time.Minute)
// - "1w" or "1week" --> one week --> time.Now().Add(-7 * 24 * time.Hour)
// - "30s" or "30second" --> thirty seconds --> time.Now().Add(-30 * time.Second)
// Note: Duration strings are case-sensitive, and should be used as mentioned above only.
// Limitation: Value of numerical multiplier, X should be in b/w 0 - 1e6 (1 million), boundary values excluded i.e.
// 0 < X < 1e6. Also, the maximum time in the past can be 1 January 1970 00:00:00 UTC (epoch time),
// so giving "1000y" will result in epoch time.
func parseTimeRange(timeRange string) (time.Time, error) {
	match, err := regexp.MatchString(defaultDateTimeRangeShortRE, timeRange)
	if !match { // fallback on to check if it's of longer notation
		_, err = regexp.MatchString(defaultDateTimeRangeLongRE, timeRange)
	}
	if err != nil {
		return time.Time{}, err
	}

	re, _ := regexp.Compile(defaultDateTimeRangeNum)
	idx := re.FindStringSubmatchIndex(timeRange)
	if idx == nil {
		return time.Time{}, fmt.Errorf("cannot parse timeRange %s", timeRange)
	}

	num, err := strconv.Atoi(timeRange[idx[0]:idx[1]])
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse timeRange %s", timeRange)
	}
	if num >= 1e6 {
		return time.Time{}, fmt.Errorf("invalid time-duation multiplier %d, allowed range is 0 < multiplier < 1000000", num)
	}

	dur, err := parseTimeDuration(timeRange[idx[1]:])
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse timeRange %s", timeRange)
	}

	res := time.Now().Add(time.Duration(-num) * dur) // using server's local timezone
	epochTime := time.Unix(0, 0)
	if res.Before(epochTime) {
		res = epochTime
	}
	return res, nil
}

func parseSingleTs(ts string) (time.Time, error) {
	var tsOut time.Time
	var err error
	formats := []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02", "2006-01-02T15:04:05+0700", time.RFC3339}
	for _, format := range formats {
		if tsOut, err = time.Parse(format, ts); err == nil {
			return tsOut, err
		}
	}
	return tsOut, err
}

// parseTimeDuration parses the given time duration in either short or long convention
// and returns the time.Duration
// Valid values (long notation/short notation):
// - second/s
// - minute/m
// - hour/h
// - day/d
// - week/w
// - month/M
// - year/y
// NOTE: the input "duration" is case-sensitive
func parseTimeDuration(duration string) (dur time.Duration, err error) {
	switch duration {
	case "s", "second":
		dur = time.Second
	case "m", "minute":
		dur = time.Minute
	case "h", "hour":
		dur = time.Hour
	case "d", "day":
		dur = day
	case "w", "week":
		dur = week
	case "M", "month":
		dur = month
	case "y", "year":
		dur = year
	default:
		err = fmt.Errorf("unknown time duration %s", duration)
	}
	return
}

// strToTaskListType converts str to types.TaskListType
// if it's not matched, it returns types.TaskListTypeDecision
func strToTaskListType(str string) types.TaskListType {
	if strings.ToLower(str) == "activity" {
		return types.TaskListTypeActivity
	}
	return types.TaskListTypeDecision
}

func getCliIdentity() string {
	return fmt.Sprintf("cadence-cli@%s", getHostName())
}

func getHostName() string {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "UnKnown"
	}
	return hostName
}

func processJWTFlags(ctx context.Context, cliCtx *cli.Context) (context.Context, error) {
	path := getJWTPrivateKey(cliCtx)
	t := getJWT(cliCtx)
	var token string
	var err error

	if t != "" {
		token = t
	} else if path != "" {
		token, err = createJWT(path, clock.NewRealTimeSource())
		if err != nil {
			return nil, fmt.Errorf("error creating JWT token: %w", err)
		}
	}

	return context.WithValue(ctx, CtxKeyJWT, token), nil
}

func populateContextFromCLIContext(ctx context.Context, cliCtx *cli.Context) (context.Context, error) {
	ctx, err := processJWTFlags(ctx, cliCtx)
	if err != nil {
		return nil, fmt.Errorf("error while populating context from CLI: %w", err)
	}
	return ctx, nil
}

func newContext(c *cli.Context) (context.Context, context.CancelFunc, error) {
	return newTimedContext(c, defaultContextTimeout)
}

func newContextForLongPoll(c *cli.Context) (context.Context, context.CancelFunc, error) {
	return newTimedContext(c, defaultContextTimeoutForLongPoll)
}

func newIndefiniteContext(c *cli.Context) (context.Context, context.CancelFunc, error) {
	if c.IsSet(FlagContextTimeout) {
		ctx, cancel, err := newTimedContext(c, time.Duration(c.Int(FlagContextTimeout))*time.Second)
		defer cancel()
		if err != nil {
			return nil, nil, fmt.Errorf("Error in new indifinite context: %w", err)
		}
		return ctx, cancel, nil
	}
	ctx, err := populateContextFromCLIContext(c.Context, c)
	if err != nil {
		return nil, nil, fmt.Errorf("error in newIndefiniteContext: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	return ctx, cancel, nil
}

func newTimedContext(c *cli.Context, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if overrideTimeout := c.Int(FlagContextTimeout); overrideTimeout > 0 {
		timeout = time.Duration(overrideTimeout) * time.Second
	}

	ctx, err := populateContextFromCLIContext(c.Context, c)
	if err != nil {
		return nil, nil, fmt.Errorf("error populate context from CLI context: %w", err)
	}
	ctx, timeOut := context.WithTimeout(ctx, timeout)

	return ctx, timeOut, nil
}

// process and validate input provided through cmd or file
func processJSONInput(c *cli.Context) (string, error) {
	return processJSONInputHelper(c, jsonTypeInput)
}

// process and validate json
func processJSONInputHelper(c *cli.Context, jType jsonType) (string, error) {
	var flagNameOfRawInput string
	var flagNameOfInputFileName string

	switch jType {
	case jsonTypeInput:
		flagNameOfRawInput = FlagInput
		flagNameOfInputFileName = FlagInputFile
	case jsonTypeMemo:
		flagNameOfRawInput = FlagMemo
		flagNameOfInputFileName = FlagMemoFile
	case jsonTypeHeader:
		flagNameOfRawInput = FlagHeaderValue
		flagNameOfInputFileName = FlagHeaderFile
	case jsonTypeSignal:
		flagNameOfRawInput = FlagSignalInput
		flagNameOfInputFileName = FlagSignalInputFile
	default:
		return "", nil
	}

	var input string
	if c.IsSet(flagNameOfRawInput) {
		input = c.String(flagNameOfRawInput)
	} else if c.IsSet(flagNameOfInputFileName) {
		inputFile := c.String(flagNameOfInputFileName)
		// This method is purely used to parse input from the CLI. The input comes from a trusted user
		// #nosec
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return "", fmt.Errorf("error reading input file: %w", err)
		}
		input = string(data)
	}
	if input != "" {
		if err := validateJSONs(input); err != nil {
			return "", fmt.Errorf("input is not valid JSON: %w", err)
		}
	}
	return input, nil
}

func processMultipleKeys(rawKey, separator string) []string {
	var keys []string
	if strings.TrimSpace(rawKey) != "" {
		keys = strings.Split(rawKey, separator)
	}
	return keys
}

func processMultipleJSONValues(rawValue string) ([]string, error) {
	var values []string
	var sc fastjson.Scanner
	sc.Init(rawValue)
	for sc.Next() {
		values = append(values, sc.Value().String())
	}
	if err := sc.Error(); err != nil {
		return nil, fmt.Errorf("parse json error: %w", err)
	}
	return values, nil
}

func mapFromKeysValues(keys, values []string) map[string][]byte {
	fields := map[string][]byte{}
	for i, key := range keys {
		fields[key] = []byte(values[i])
	}
	return fields
}

// validate whether str is a valid json or multi valid json concatenated with spaces/newlines
func validateJSONs(str string) error {
	input := []byte(str)
	dec := json.NewDecoder(bytes.NewReader(input))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return nil // End of input, valid JSON
		}
		if err != nil {
			return err // Invalid input
		}
	}
}

// use parseBool to ensure all BOOL search attributes only be "true" or "false"
func parseBool(str string) (bool, error) {
	switch str {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, fmt.Errorf("not parseable bool value: %s", str)
}

func trimSpace(strs []string) []string {
	result := make([]string, len(strs))
	for i, v := range strs {
		result[i] = strings.TrimSpace(v)
	}
	return result
}

func parseArray(v string) (interface{}, error) {
	if len(v) > 0 && v[0] == '[' && v[len(v)-1] == ']' {
		parsedValues, err := fastjson.Parse(v)
		if err != nil {
			return nil, err
		}
		arr, err := parsedValues.Array()
		if err != nil {
			return nil, err
		}
		result := make([]interface{}, len(arr))
		for i, item := range arr {
			s := item.String()
			if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' { // remove addition quote from json
				s = s[1 : len(s)-1]
				if sTime, err := time.Parse(defaultDateTimeFormat, s); err == nil {
					result[i] = sTime
					continue
				}
			}
			result[i] = s
		}
		return result, nil
	}
	return nil, errors.New("not array")
}

func convertStringToRealType(v string) interface{} {
	var genVal interface{}
	var err error

	if genVal, err = strconv.ParseInt(v, 10, 64); err == nil {

	} else if genVal, err = parseBool(v); err == nil {

	} else if genVal, err = strconv.ParseFloat(v, 64); err == nil {

	} else if genVal, err = time.Parse(defaultDateTimeFormat, v); err == nil {

	} else if genVal, err = parseArray(v); err == nil {

	} else {
		genVal = v
	}

	return genVal
}

func truncate(str string) string {
	if len(str) > maxOutputStringLength {
		return str[:maxOutputStringLength]
	}
	return str
}

// this only works for ANSI terminal, which means remove existing lines won't work if users redirect to file
// ref: https://en.wikipedia.org/wiki/ANSI_escape_code
func removePrevious2LinesFromTerminal(output io.Writer) {
	output.Write([]byte("\033[1A"))
	output.Write([]byte("\033[2K"))
	output.Write([]byte("\033[1A"))
	output.Write([]byte("\033[2K"))
}

func showNextPage(output io.Writer) bool {
	output.Write([]byte(fmt.Sprintf("Press %s to show next page, press %s to quit: ",
		color.GreenString("Enter"), color.RedString("any other key then Enter"))))
	var input string
	fmt.Scanln(&input)
	return strings.Trim(input, " ") == ""
}

// prompt will show input msg, then waiting user input y/yes to continue
func prompt(msg string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println(msg)
	text, _ := reader.ReadString('\n')
	textLower := strings.ToLower(strings.TrimRight(text, "\n"))
	if textLower != "y" && textLower != "yes" {
		os.Exit(0)
	}
}
func getInputFile(inputFile string) (*os.File, error) {
	if len(inputFile) == 0 {
		info, err := os.Stdin.Stat()
		if err != nil {
			return nil, fmt.Errorf("failed to stat stdin file handle: %w", err)
		}
		if info.Mode()&os.ModeCharDevice != 0 || info.Size() <= 0 {
			fmt.Fprintln(os.Stderr, "Provide a filename or pass data to STDIN")
			os.Exit(1)
		}
		return os.Stdin, nil
	}
	// This code is executed from the CLI. All user input is from a CLI user.
	// #nosec
	f, err := os.Open(inputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file for reading: %v: %w", inputFile, err)
	}
	return f, nil
}

// createJWT defines the logic to create a JWT
func createJWT(keyPath string, timeSource clock.TimeSource) (string, error) {
	privateKey, err := common.LoadRSAPrivateKey(keyPath)
	if err != nil {
		return "", err
	}

	ttl := int64(60 * 10)
	claims := authorization.JWTClaims{
		Admin: true,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(timeSource.Now()),
			ExpiresAt: jwt.NewNumericDate(timeSource.Now().Add(time.Second * time.Duration(ttl))),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
}

func getWorkflowMemo(input map[string]interface{}) (*types.Memo, error) {
	if input == nil {
		return nil, nil
	}

	memo := make(map[string][]byte)
	for k, v := range input {
		memoBytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("encode workflow memo error: %w", err)
		}
		memo[k] = memoBytes
	}
	return &types.Memo{Fields: memo}, nil
}

func serializeSearchAttributes(input map[string]interface{}) (*types.SearchAttributes, error) {
	if input == nil {
		return nil, nil
	}

	attr := make(map[string][]byte)
	for k, v := range input {
		attrBytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("encode search attribute [%s] error: %v", k, err)
		}
		attr[k] = attrBytes
	}
	return &types.SearchAttributes{IndexedFields: attr}, nil
}

// parseIntMultiRange will parse string of multiple integer ranges separates by commas.
// Single range can be an integer or inclusive range separated by dash.
// The result is a sorted set union of integers.
// Example: "3,8-8,5-6" -> [3,4,5,8]
func parseIntMultiRange(s string) ([]int, error) {
	set := map[int]struct{}{}
	ranges := strings.Split(strings.TrimSpace(s), ",")
	for _, r := range ranges {
		r = strings.TrimSpace(r)
		if len(r) == 0 {
			continue
		}
		parts := strings.Split(r, "-")
		switch len(parts) {
		case 1:
			i, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("single number %q: %v", r, err)
			}
			set[i] = struct{}{}
		case 2:
			lower, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("lower range of %q: %v", r, err)
			}
			upper, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("upper range of %q: %v", r, err)
			}
			for i := lower; i <= upper; i++ {
				set[i] = struct{}{}
			}
		default:
			return nil, fmt.Errorf("invalid range %q", r)
		}
	}

	result := []int{}
	for i := range set {
		result = append(result, i)
	}
	sort.Ints(result)
	return result, nil
}
