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
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
	"github.com/stretchr/testify/require"
)

type (
	os2Client struct {
		client *opensearch.Client
	}
)

func newOS2Client(url string) (*os2Client, error) {

	esClient, err := opensearch.NewClient(
		opensearch.Config{
			Addresses:    []string{url},
			MaxRetries:   5,
			RetryBackoff: func(i int) time.Duration { return time.Duration(i) * 100 * time.Millisecond },
		},
	)

	return &os2Client{
		client: esClient,
	}, err
}

func (os2 *os2Client) PutIndexTemplate(t *testing.T, templateConfigFile, templateName string) {
	// This function is used exclusively in tests. Excluding it from security checks.
	// #nosec
	template, err := os.Open(templateConfigFile)
	require.NoError(t, err)

	req := opensearchapi.IndicesPutIndexTemplateRequest{
		Body: template,
		Name: templateName,
	}

	ctx, cancel := createContext()
	defer cancel()
	resp, err := req.Do(ctx, os2.client)
	require.NoError(t, err)
	require.False(t, resp.IsError(), fmt.Sprintf("OS2 error: %s", resp.String()))
	resp.Body.Close()

}

func (os2 *os2Client) CreateIndex(t *testing.T, indexName string) {
	existsReq := opensearchapi.IndicesExistsRequest{
		Index: []string{indexName},
	}
	ctx, cancel := createContext()
	defer cancel()
	resp, err := existsReq.Do(ctx, os2.client)
	require.NoError(t, err)

	if resp.StatusCode == http.StatusOK {
		deleteReq := opensearchapi.IndicesDeleteRequest{
			Index: []string{indexName},
		}
		ctx, cancel := createContext()
		defer cancel()
		resp, err := deleteReq.Do(ctx, os2.client)
		require.Nil(t, err)
		require.False(t, resp.IsError(), fmt.Sprintf("OS2 error: %s", resp.String()))
	}

	resp.Body.Close()

	createReq := opensearchapi.IndicesCreateRequest{
		Index: indexName,
	}

	ctx, cancel = createContext()
	defer cancel()
	resp, err = createReq.Do(ctx, os2.client)
	require.NoError(t, err)
	require.False(t, resp.IsError(), fmt.Sprintf("OS2 error: %s", resp.String()))
	resp.Body.Close()
}

func (os2 *os2Client) DeleteIndex(t *testing.T, indexName string) {
	deleteReq := opensearchapi.IndicesDeleteRequest{
		Index: []string{indexName},
	}
	ctx, cancel := createContext()
	defer cancel()
	resp, err := deleteReq.Do(ctx, os2.client)
	require.NoError(t, err)
	require.False(t, resp.IsError(), fmt.Sprintf("OS2 error: %s", resp.String()))
	resp.Body.Close()

}

func (os2 *os2Client) PutMaxResultWindow(t *testing.T, indexName string, maxResultWindow int) error {

	req := opensearchapi.IndicesPutSettingsRequest{
		Body:  strings.NewReader(fmt.Sprintf(`{"max_result_window" : %d}`, maxResultWindow)),
		Index: []string{indexName},
	}

	ctx, cancel := createContext()
	defer cancel()
	res, err := req.Do(ctx, os2.client)
	require.NoError(t, err)

	res.Body.Close()

	return nil
}

func (os2 *os2Client) GetMaxResultWindow(t *testing.T, indexName string) (string, error) {

	req := opensearchapi.IndicesGetSettingsRequest{
		Index: []string{indexName},
	}

	ctx, cancel := createContext()
	defer cancel()
	res, err := req.Do(ctx, os2.client)
	require.NoError(t, err)
	defer res.Body.Close()

	if res.IsError() {
		return "", errors.New(res.String())
	}

	var data map[string]map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&data)
	require.NoError(t, err)

	if info, ok := data[indexName]["settings"].(map[string]interface{}); ok {
		return info["index"].(map[string]interface{})["max_result_window"].(string), nil
	}

	return "", fmt.Errorf("no settings for index %q", indexName)

}
