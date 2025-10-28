package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/golang/snappy"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

var (
	// snappyMagic is a magic prefix prepended to compressed data to distinguish it from uncompressed data
	snappyMagic = []byte{0xff, 0x06, 0x00, 0x00, 's', 'N', 'a', 'P', 'p', 'Y'}
)

// Compress compresses data using snappy's framed format if compression is enabled
// If compressionEnabled returns false, returns the data as-is without compression
func Compress(data []byte, compressionEnabled bool) ([]byte, error) {
	if !compressionEnabled {
		return data, nil
	}

	var buf bytes.Buffer
	w := snappy.NewBufferedWriter(&buf)

	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
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

	if !hasFramedHeader(data) {
		return data, nil
	}
	r := snappy.NewReader(bytes.NewReader(data))
	decompressed, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return decompressed, nil
}

// CompressedActiveStatus returns the compressed active status string.
func CompressedActiveStatus(compressionEnabled bool) string {
	compressed, _ := Compress([]byte(fmt.Sprintf(`"%s"`, types.ExecutorStatusACTIVE)), compressionEnabled)
	return string(compressed)
}

// DecompressAndUnmarshal decompresses data and unmarshals it into the target
// errorContext is used to provide meaningful error messages
func DecompressAndUnmarshal(data []byte, target interface{}, errorContext string, logger log.Logger) error {
	decompressed, err := Decompress(data)
	if err != nil {
		return fmt.Errorf("decompress %s: %w", errorContext, err)
	}
	if err := json.Unmarshal(decompressed, target); err != nil {
		return fmt.Errorf("unmarshal %s: %w", errorContext, err)
	}
	return nil
}

func hasFramedHeader(b []byte) bool {
	return len(b) >= len(snappyMagic) && bytes.Equal(b[:len(snappyMagic)], snappyMagic)
}
