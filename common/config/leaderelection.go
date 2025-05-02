// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package config

import "time"

// LeaderElection is a configuration for leader election running.
type LeaderElection struct {
	Store      LeaderStore   `yaml:"leaderStore"`
	Election   Election      `yaml:"election"`
	Namespaces []Namespace   `yaml:"namespaces"`
	Process    LeaderProcess `yaml:"process"`
}

// LeaderStore provides a config for leader election.
type LeaderStore struct {
	ETCD ETCD `yaml:"etcd"`
}

type Namespace struct {
	Name string `yaml:"name"`
	Mode string `yaml:"mode"` // TODO: this should an ENUM with possible modes: enabled, read_only, proxy, disabled
}

type Election struct {
	Prefix                 string        `yaml:"prefix"`
	TTL                    time.Duration `yaml:"ttl"`
	RenewDelta             time.Duration `yaml:"renewDelta"`
	LeaderPeriod           time.Duration `yaml:"leaderPeriod"`           // Time to hold leadership before resigning
	MaxRandomDelay         time.Duration `yaml:"maxRandomDelay"`         // Maximum random delay before campaigning
	FailedElectionCooldown time.Duration `yaml:"failedElectionCooldown"` // wait between election attempts with unhandled errors
}

type LeaderProcess struct {
	Period time.Duration `yaml:"period"`
}
