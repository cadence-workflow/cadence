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

package service

import "strings"

const (
	_servicePrefix = "cadence-"

	// Frontend is the name of the frontend service
	Frontend = "cadence-frontend"
	// History is the name of the history service
	History = "cadence-history"
	// Matching is the name of the matching service
	Matching = "cadence-matching"
	// Worker is the name of the worker service
	Worker = "cadence-worker"
	// ShardDistributor is the name of the shard distributor service
	ShardDistributor = "cadence-shard-distributor"
)

// ListWithRing contains the list of all cadence services that has a hash ring
var ListWithRing = []string{Frontend, History, Matching, Worker}

// List contains the list of all cadence services
var List = []string{Frontend, History, Matching, Worker, ShardDistributor}

// ShortName returns cadence service name without "cadence-" prefix
func ShortName(name string) string {
	return strings.TrimPrefix(name, _servicePrefix)
}

// FullName returns cadence service name with "cadence-" prefix
func FullName(name string) string {
	if strings.HasPrefix(name, _servicePrefix) {
		return name
	}
	return _servicePrefix + name
}

func ShortNames(names []string) []string {
	result := make([]string, len(names))
	for i := range names {
		result[i] = ShortName(names[i])
	}
	return result
}
