package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/golang/snappy"
)

var (
	// _snappyHeader is a magic prefix prepended to compressed data to distinguish it from uncompressed data
	_snappyHeader = []byte{0xff, 0x06, 0x00, 0x00, 's', 'N', 'a', 'P', 'p', 'Y'}
)

const (
	// CompressionSnappy indicates snappy compression should be applied
	CompressionSnappy = "snappy"
)

type compressionMode int

const (
	compressionNone compressionMode = iota
	compressionSnappy
)

// RecordWriter handles serialization of data for etcd, applying compression when configured.
type RecordWriter struct {
	mode compressionMode
}

// NewRecordWriter constructs a RecordWriter based on the configured compression type.
func NewRecordWriter(compressionType string) (*RecordWriter, error) {
	switch strings.ToLower(compressionType) {
	case "", "none":
		return &RecordWriter{mode: compressionNone}, nil
	case CompressionSnappy:
		return &RecordWriter{mode: compressionSnappy}, nil
	default:
		return nil, fmt.Errorf("unsupported compression type: %s", compressionType)
	}
}

// Write serializes data using the configured compression mode.
func (w *RecordWriter) Write(data []byte) ([]byte, error) {
	if w.mode == compressionNone {
		return data, nil
	}

	var buf bytes.Buffer
	writer := snappy.NewBufferedWriter(&buf)

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decompress decodes snappy-compressed data
// If the snappy header is present, it will successfully decompress it or return an error
// If the snappy header is absent, it treats data as uncompressed and returns it as-is
func Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	if !bytes.HasPrefix(data, _snappyHeader) {
		return data, nil
	}

	r := snappy.NewReader(bytes.NewReader(data))
	decompressed, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return decompressed, nil
}

// DecompressAndUnmarshal decompresses data and unmarshals it into the target
func DecompressAndUnmarshal(data []byte, target interface{}) error {
	decompressed, err := Decompress(data)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}

	if err := json.Unmarshal(decompressed, target); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	return nil
}
