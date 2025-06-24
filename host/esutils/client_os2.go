// Copyright (c) 2020 Uber Technologies, Inc.
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

package esutils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4"
	osapi "github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/require"
)

type (
	os2Client struct {
		client *osapi.Client
	}
)

func newOS2Client(url string) (*os2Client, error) {

	osClient, err := osapi.NewClient(osapi.Config{
		opensearch.Config{
			Addresses:    []string{url},
			MaxRetries:   5,
			RetryBackoff: func(i int) time.Duration { return time.Duration(i) * 100 * time.Millisecond },
		},
	},
	)

	return &os2Client{
		client: osClient,
	}, err
}

func (os2 *os2Client) PutIndexTemplate(t *testing.T, templateConfigFile, templateName string) {
	// This function is used exclusively in tests. Excluding it from security checks.
	// #nosec
	template, err := os.Open(templateConfigFile)
	require.NoError(t, err)

	req := osapi.IndexTemplateCreateReq{
		Body:          template,
		IndexTemplate: templateName,
	}

	ctx, cancel := createContext()
	defer cancel()
	resp, err := os2.client.IndexTemplate.Create(ctx, req)
	require.NoError(t, err)
	require.True(t, resp.Acknowledged, fmt.Sprintf("OS2 put index template unacknowledged: %s", resp.Inspect().Response.Body))
}

func (os2 *os2Client) CreateIndex(t *testing.T, indexName string) {
	existsReq := osapi.IndicesExistsReq{
		Indices: []string{indexName},
	}
	ctx, cancel := createContext()
	defer cancel()
	resp, err := os2.client.Indices.Exists(ctx, existsReq)
	require.NoError(t, err)

	if resp.StatusCode == http.StatusOK {
		deleteReq := osapi.IndicesDeleteReq{
			Indices: []string{indexName},
		}
		ctx, cancel := createContext()
		defer cancel()
		resp, err := os2.client.Indices.Delete(ctx, deleteReq)
		require.Nil(t, err)
		require.True(t, resp.Acknowledged, fmt.Sprintf("OS2 delete index unacknowledged: %s", resp.Inspect().Response.Body))
	}

	resp.Body.Close()

	createReq := osapi.IndicesCreateReq{
		Index: indexName,
	}

	ctx, cancel = createContext()
	defer cancel()
	createResp, err := os2.client.Indices.Create(ctx, createReq)
	require.NoError(t, err)
	require.True(t, createResp.Acknowledged, fmt.Sprintf("OS2 create index unacknowledged: %s", createResp.Inspect().Response.Body))
}

func (os2 *os2Client) DeleteIndex(t *testing.T, indexName string) {
	deleteReq := osapi.IndicesDeleteReq{
		Indices: []string{indexName},
	}
	ctx, cancel := createContext()
	defer cancel()
	resp, err := os2.client.Indices.Delete(ctx, deleteReq)
	require.NoError(t, err)
	require.True(t, resp.Acknowledged, fmt.Sprintf("OS2 delete index unacknowledged: %s", resp.Inspect().Response.Body))
}

func (os2 *os2Client) PutMaxResultWindow(t *testing.T, indexName string, maxResultWindow int) error {

	req := osapi.SettingsPutReq{
		Body:    strings.NewReader(fmt.Sprintf(`{"max_result_window" : %d}`, maxResultWindow)),
		Indices: []string{indexName},
	}

	ctx, cancel := createContext()
	defer cancel()
	resp, err := os2.client.Indices.Settings.Put(ctx, req)
	require.NoError(t, err)
	require.True(t, resp.Acknowledged, fmt.Sprintf("OS2 put index settings unacknowledged: %s", resp.Inspect().Response.Body))

	return nil
}

func (os2 *os2Client) GetMaxResultWindow(t *testing.T, indexName string) (string, error) {

	req := &osapi.SettingsGetReq{
		Indices: []string{indexName},
	}

	ctx, cancel := createContext()
	defer cancel()
	res, err := os2.client.Indices.Settings.Get(ctx, req)
	require.NoError(t, err)

	if indexSettings, ok := res.Indices[indexName]; ok {
		var settingsData map[string]interface{}
		err = json.Unmarshal(indexSettings.Settings, &settingsData)
		require.NoError(t, err)
		return settingsData["max_result_window"].(string), nil
	}

	return "", fmt.Errorf("no settings for index %q", indexName)
}
