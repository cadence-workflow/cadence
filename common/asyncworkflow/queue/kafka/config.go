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

package kafka

import (
	"fmt"
	"strings"

	"github.com/Shopify/sarama"

	"github.com/uber/cadence/common/authorization"
	"github.com/uber/cadence/common/config"
)

type (
	queueConfig struct {
		Connection connectionConfig `yaml:"connection"`
		Topic      string           `yaml:"topic"`
	}

	connectionConfig struct {
		Brokers []string    `yaml:"brokers"`
		TLS     config.TLS  `yaml:"tls"`
		SASL    config.SASL `yaml:"sasl"`
	}
)

func (c *queueConfig) ID() string {
	return fmt.Sprintf("kafka::%s/%s", c.Topic, strings.Join(c.Connection.Brokers, ","))
}

func newSaramaConfigWithAuth(tls *config.TLS, sasl *config.SASL) (*sarama.Config, error) {
	saramaConfig := sarama.NewConfig()

	// TLS support
	tlsConfig, err := tls.ToTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("Error creating Kafka TLS config %w", err)
	}
	if tlsConfig != nil {
		saramaConfig.Net.TLS.Enable = true
		saramaConfig.Net.TLS.Config = tlsConfig
	}

	// SASL support
	if sasl.Enabled {
		saramaConfig.Net.SASL.Enable = true
		saramaConfig.Net.SASL.Handshake = true
		saramaConfig.Net.SASL.User = sasl.User
		saramaConfig.Net.SASL.Password = sasl.Password
		switch sasl.Algorithm {
		case "sha512":
			saramaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &authorization.XDGSCRAMClient{HashGeneratorFcn: authorization.SHA512}
			}
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		case "sha256":
			saramaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &authorization.XDGSCRAMClient{HashGeneratorFcn: authorization.SHA256}
			}
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		case "plain":
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		default:
			return nil, fmt.Errorf("unknown SASL algorithm %v", sasl.Algorithm)
		}
	}
	return saramaConfig, nil
}
