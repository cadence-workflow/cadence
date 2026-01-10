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
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/olivere/elastic"
	"github.com/urfave/cli/v2"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/.gen/go/indexer"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/elasticsearch"
	es "github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/elasticsearch/esql"
	"github.com/uber/cadence/tools/common/commoncli"
)

const (
	versionTypeExternal = "external"
)

var timeKeys = map[string]bool{
	"StartTime":     true,
	"CloseTime":     true,
	"ExecutionTime": true,
}

func timeKeyFilter(key string) bool {
	return timeKeys[key]
}

func timeValProcess(timeStr string) (string, error) {
	// first check if already in int64 format
	if _, err := strconv.ParseInt(timeStr, 10, 64); err == nil {
		return timeStr, nil
	}
	// try to parse time
	parsedTime, err := time.Parse(defaultDateTimeFormat, timeStr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", parsedTime.UnixNano()), nil
}

type ESIndexRow struct {
	Health             string `header:"Health"`
	Status             string `header:"Status"`
	Index              string `header:"Index"`
	PrimaryShards      int    `header:"Pri"`
	ReplicaShards      int    `header:"Rep"`
	DocsCount          int    `header:"Docs Count"`
	DocsDeleted        int    `header:"Docs Deleted"`
	StorageSize        string `header:"Store Size"`
	PrimaryStorageSize string `header:"Pri Store Size"`
}

// AdminCatIndices cat indices for ES cluster
func AdminCatIndices(c *cli.Context) error {
	esClient, err := getDeps(c).ElasticSearchClient(c)
	if err != nil {
		return err
	}

	ctx := c.Context
	resp, err := esClient.CatIndices().Do(ctx)
	if err != nil {
		return commoncli.Problem("Unable to cat indices", err)
	}

	table := []ESIndexRow{}
	for _, row := range resp {
		table = append(table, ESIndexRow{
			Health:             row.Health,
			Status:             row.Status,
			Index:              row.Index,
			PrimaryShards:      row.Pri,
			ReplicaShards:      row.Rep,
			DocsCount:          row.DocsCount,
			DocsDeleted:        row.DocsDeleted,
			StorageSize:        row.StoreSize,
			PrimaryStorageSize: row.PriStoreSize,
		})
	}
	return Render(c, table, RenderOptions{DefaultTemplate: templateTable, Color: true, Border: true})
}

// AdminIndex used to bulk insert message from kafka parse
func AdminIndex(c *cli.Context) error {
	esClient, err := getDeps(c).ElasticSearchClient(c)
	if err != nil {
		return err
	}
	indexName, err := getRequiredOption(c, FlagIndex)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	inputFileName, err := getRequiredOption(c, FlagInputFile)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	batchSize := c.Int(FlagBatchSize)

	messages, err := parseIndexerMessage(inputFileName)
	if err != nil {
		return commoncli.Problem("Unable to parse indexer message", err)
	}

	bulkRequest := esClient.Bulk()
	bulkConductFn := func() error {
		_, err := bulkRequest.Do(c.Context)
		if err != nil {
			return commoncli.Problem("Bulk failed", err)
		}
		if bulkRequest.NumberOfActions() != 0 {
			return commoncli.Problem(fmt.Sprintf("Bulk request not done, %d", bulkRequest.NumberOfActions()), err)
		}
		return nil
	}
	for i, message := range messages {
		docID := message.GetWorkflowID() + elasticsearch.GetESDocDelimiter() + message.GetRunID()
		var req elastic.BulkableRequest
		switch message.GetMessageType() {
		case indexer.MessageTypeIndex:
			doc, err := generateESDoc(message)
			if err != nil {
				return commoncli.Problem("Error in Admin Index: ", err)
			}
			req = elastic.NewBulkIndexRequest().
				Index(indexName).
				Type(elasticsearch.GetESDocType()).
				Id(docID).
				VersionType(versionTypeExternal).
				Version(message.GetVersion()).
				Doc(doc)
		case indexer.MessageTypeDelete:
			req = elastic.NewBulkDeleteRequest().
				Index(indexName).
				Type(elasticsearch.GetESDocType()).
				Id(docID).
				VersionType(versionTypeExternal).
				Version(message.GetVersion())
		case indexer.MessageTypeCreate:
			req = elastic.NewBulkIndexRequest().
				OpType("create").
				Index(indexName).
				Type(elasticsearch.GetESDocType()).
				Id(docID).
				VersionType("internal")
		default:
			return commoncli.Problem("Unknown message type", nil)
		}
		bulkRequest.Add(req)

		if i%batchSize == batchSize-1 {
			if err := bulkConductFn(); err != nil {
				return err
			}
		}
	}
	if bulkRequest.NumberOfActions() != 0 {
		if err := bulkConductFn(); err != nil {
			return err
		}
	}
	return nil
}

// AdminDelete used to delete documents from ElasticSearch with input of list result
func AdminDelete(c *cli.Context) error {
	esClient, err := getDeps(c).ElasticSearchClient(c)
	if err != nil {
		return err
	}
	indexName, err := getRequiredOption(c, FlagIndex)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	inputFileName, err := getRequiredOption(c, FlagInputFile)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	batchSize := c.Int(FlagBatchSize)
	rps := c.Int(FlagRPS)
	ratelimiter := clock.NewRatelimiter(rate.Limit(rps), rps)

	// This is only executed from the CLI by an admin user
	// #nosec
	file, err := os.Open(inputFileName)
	if err != nil {
		return commoncli.Problem("Cannot open input file", nil)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	scanner.Scan() // skip first line
	i := 0

	bulkRequest := esClient.Bulk()
	bulkConductFn := func() error {
		// Wait for rate limiter token to become available
		if err = ratelimiter.Wait(c.Context); err != nil {
			return err
		}
		_, err = bulkRequest.Do(c.Context)
		if err != nil {
			return commoncli.Problem(fmt.Sprintf("Bulk failed, current processed row %d", i), err)
		}
		if bulkRequest.NumberOfActions() != 0 {
			return commoncli.Problem(fmt.Sprintf("Bulk request not done, current processed row %d", i), err)
		}
		return nil
	}

	for scanner.Scan() {
		line := strings.Split(scanner.Text(), "|")
		docID := strings.TrimSpace(line[1]) + elasticsearch.GetESDocDelimiter() + strings.TrimSpace(line[2])
		req := elastic.NewBulkDeleteRequest().
			Index(indexName).
			Type(elasticsearch.GetESDocType()).
			Id(docID).
			VersionType(versionTypeExternal).
			Version(math.MaxInt64)
		bulkRequest.Add(req)
		if i%batchSize == batchSize-1 {
			if err = bulkConductFn(); err != nil {
				return err
			}
		}
		i++
	}
	if bulkRequest.NumberOfActions() != 0 {
		return bulkConductFn()
	}
	return nil
}

func parseIndexerMessage(fileName string) (messages []*indexer.Message, err error) {
	// Executed from the CLI to parse existing elastiseach files
	// #nosec
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	idx := 0
	for scanner.Scan() {
		idx++
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			fmt.Printf("line %v is empty, skipped\n", idx)
			continue
		}

		msg := &indexer.Message{}
		err := json.Unmarshal([]byte(line), msg)
		if err != nil {
			fmt.Printf("line %v cannot be deserialized to indexer message: %v.\n", idx, line)
			return nil, err
		}
		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func generateESDoc(msg *indexer.Message) (map[string]interface{}, error) {
	doc := make(map[string]interface{})
	doc[es.DomainID] = msg.GetDomainID()
	doc[es.WorkflowID] = msg.GetWorkflowID()
	doc[es.RunID] = msg.GetRunID()

	for k, v := range msg.Fields {
		switch v.GetType() {
		case indexer.FieldTypeString:
			doc[k] = v.GetStringData()
		case indexer.FieldTypeInt:
			doc[k] = v.GetIntData()
		case indexer.FieldTypeBool:
			doc[k] = v.GetBoolData()
		case indexer.FieldTypeBinary:
			doc[k] = v.GetBinaryData()
		default:
			return nil, fmt.Errorf("Unknown field type %v", nil)
		}
	}
	return doc, nil
}

// This function is used to trim unnecessary tag in returned json for table header
func trimBucketKey(k string) string {
	// group key is in form of "group_key", we only need "key" as the column name
	k = strings.TrimPrefix(k, "group_")
	k = strings.TrimPrefix(k, "Attr_")
	return fmt.Sprintf(`%v(*)`, k)
}

// parse the returned time to readable string if time is in int64 format
func toTimeStr(s interface{}) string {
	floatTime, err := strconv.ParseFloat(s.(string), 64)
	intTime := int64(floatTime)
	if err != nil {
		return s.(string)
	}
	t := time.Unix(0, intTime)
	return t.Format(time.RFC3339)
}

// GenerateReport generate report for an aggregation query to ES
func GenerateReport(c *cli.Context) error {
	output := getDeps(c).Output()

	// use url command argument to create client
	index, err := getRequiredOption(c, FlagIndex)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	sql, err := getRequiredOption(c, FlagListQuery)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	var reportFormat, reportFilePath string
	if c.IsSet(FlagOutputFormat) {
		reportFormat = c.String(FlagOutputFormat)
	}
	if c.IsSet(FlagOutputFilename) {
		reportFilePath = c.String(FlagOutputFilename)
	} else {
		reportFilePath = "./report." + reportFormat
	}
	esClient, err := getDeps(c).ElasticSearchClient(c)
	if err != nil {
		return err
	}
	ctx := c.Context

	// convert sql to dsl
	e := esql.NewESql()
	e.SetCadence(true)
	e.ProcessQueryValue(timeKeyFilter, timeValProcess)
	dsl, sortFields, err := e.ConvertPrettyCadence(sql, "")
	if err != nil {
		return commoncli.Problem("Fail to convert sql to dsl", err)
	}

	// query client
	resp, err := esClient.Search(index).Source(dsl).Do(ctx)
	if err != nil {
		return commoncli.Problem("Fail to talk with ES", err)
	}

	// Show result to terminal
	table := tablewriter.NewWriter(output)
	var headers []string
	var groupby, bucket map[string]interface{}
	var buckets []interface{}
	err = json.Unmarshal(*resp.Aggregations["groupby"], &groupby)
	if err != nil {
		return commoncli.Problem("Fail to parse groupby", err)
	}
	buckets = groupby["buckets"].([]interface{})
	if len(buckets) == 0 {
		output.Write([]byte("no matching bucket\n"))
		return nil
	}

	// get the FIRST bucket in bucket list to extract all tags. These extracted tags are to be used as table heads
	bucket = buckets[0].(map[string]interface{})
	// record the column position in the table of each returned item
	ids := make(map[string]int)
	// We want these 3 columns shows at leftmost of the table in cadence report usage. It can be changed in future.
	primaryCols := []string{"group_DomainID", "group_WorkflowType", "group_CloseStatus"}
	primaryColsMap := map[string]int{
		"group_DomainID":     1,
		"group_WorkflowType": 1,
		"group_CloseStatus":  1,
	}
	buckKeys := 0 // number of bucket keys, used for table collapsing in html report
	if v, exist := bucket["key"]; exist {
		vmap := v.(map[string]interface{})
		// first search whether primaryCols keys exist, if found, put them at the table beginning
		for _, k := range primaryCols {
			if _, exist := vmap[k]; exist {
				k = trimBucketKey(k) // trim the unnecessary prefix
				headers = append(headers, k)
				ids[k] = len(ids)
				buckKeys++
			}
		}
		// extract all remaining bucket keys
		for k := range vmap {
			if _, exist := primaryColsMap[k]; !exist {
				k = trimBucketKey(k)
				headers = append(headers, k)
				ids[k] = len(ids)
				buckKeys++
			}
		}
	}
	// extract all other non-key items and set the table head accordingly
	for k := range bucket {
		if k != "key" {
			if k == "doc_count" {
				k = "count"
			}
			headers = append(headers, k)
			ids[k] = len(ids)
		}
	}
	table.SetHeader(headers)

	// read each bucket and fill the table, use map ids to find the correct spot
	var tableData [][]string
	for _, b := range buckets {
		bucket = b.(map[string]interface{})
		data := make([]string, len(headers))
		for k, v := range bucket {
			switch k {
			case "key": // fill group key
				vmap := v.(map[string]interface{})
				for kk, vv := range vmap {
					kk = trimBucketKey(kk)
					data[ids[kk]] = fmt.Sprintf("%v", vv)
				}
			case "doc_count": // fill bucket size count
				data[ids["count"]] = fmt.Sprintf("%v", v)
			default:
				var datum string
				vmap := v.(map[string]interface{})
				if strings.Contains(k, "Attr_CustomDatetimeField") {
					datum = fmt.Sprintf("%v", vmap["value_as_string"])
				} else {
					datum = fmt.Sprintf("%v", vmap["value"])
					// convert Cadence stored time (unix nano) to readable format
					if strings.Contains(k, "Time") && !strings.Contains(k, "Attr_") {
						datum = toTimeStr(datum)
					}
				}
				data[ids[k]] = datum
			}
		}
		table.Append(data)
		tableData = append(tableData, data)
	}
	table.Render()

	switch reportFormat {
	case "html", "HTML":
		sorted := len(sortFields) > 0 || strings.Contains(sql, "ORDER BY") || strings.Contains(sql, "order by")
		return generateHTMLReport(reportFilePath, buckKeys, sorted, headers, tableData)
	case "csv", "CSV":
		return generateCSVReport(reportFilePath, headers, tableData)
	default:
		return commoncli.Problem(fmt.Sprintf(`Report format %v not supported.`, reportFormat), nil)
	}
}

func generateCSVReport(reportFileName string, headers []string, tableData [][]string) error {
	// write csv report
	f, err := os.Create(reportFileName)
	if err != nil {
		return commoncli.Problem("Fail to create csv report file", err)
	}
	csvContent := strings.Join(headers, ",") + "\n"
	for _, data := range tableData {
		csvContent += strings.Join(data, ",") + "\n"
	}
	_, err = f.WriteString(csvContent)
	if err != nil {
		fmt.Printf("Error write to file, err: %v", err)
	}
	return f.Close()
}

func generateHTMLReport(reportFileName string, numBuckKeys int, sorted bool, headers []string, tableData [][]string) error {
	// write html report
	f, err := os.Create(reportFileName)
	if err != nil {
		return commoncli.Problem("Fail to create html report file", err)
	}
	var htmlContent string
	m, n := len(headers), len(tableData)
	rowSpan := make([]int, m) // record the collapsing size of each column
	for i := 0; i < m; i++ {
		rowSpan[i] = 1
		cell := wrapWithTag(headers[i], "td", "")
		htmlContent += cell
	}
	htmlContent = wrapWithTag(htmlContent, "tr", "")

	for row := 0; row < n; row++ {
		var rowData string
		for col := 0; col < m; col++ {
			rowSpan[col]--
			// don't do collapsing if sorted
			if col < numBuckKeys-1 && !sorted {
				if rowSpan[col] == 0 {
					for i := row; i < n; i++ {
						if tableData[i][col] == tableData[row][col] {
							rowSpan[col]++
						} else {
							break
						}
					}
					var property string
					if rowSpan[col] > 1 {
						property = fmt.Sprintf(`rowspan="%d"`, rowSpan[col])
					}
					cell := wrapWithTag(tableData[row][col], "td", property)
					rowData += cell
				}
			} else {
				cell := wrapWithTag(tableData[row][col], "td", "")
				rowData += cell
			}
		}
		rowData = wrapWithTag(rowData, "tr", "")
		htmlContent += rowData
	}
	htmlContent = wrapWithTag(htmlContent, "table", "")
	htmlContent = wrapWithTag(htmlContent, "body", "")
	htmlContent = wrapWithTag(htmlContent, "html", "")

	//nolint:errcheck
	f.WriteString("<!DOCTYPE html>\n")
	f.WriteString(`<head>
	<style>
	table, th, td {
	  border: 1px solid black;
	  padding: 5px;
	}
	table {
	  border-spacing: 2px;
	}
	</style>
	</head>` + "\n")
	f.WriteString(htmlContent)
	return f.Close()
}

// return a string that use tag to wrap content
func wrapWithTag(content string, tag string, property string) string {
	if property != "" {
		property = " " + property
	}
	if tag != "td" {
		content = "\n" + content
	}
	return "<" + tag + property + ">" + content + "</" + tag + ">\n"
}
