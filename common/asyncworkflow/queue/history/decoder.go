package history

import (
	"encoding/json"
	"fmt"

	"github.com/uber/cadence/common/asyncworkflow/queue/provider"
	"github.com/uber/cadence/common/types"
)

type decoderImpl struct {
	blob *types.DataBlob
}

func newDecoder(blob *types.DataBlob) provider.Decoder {
	return &decoderImpl{
		blob: blob,
	}
}

func (d *decoderImpl) Decode(out any) error {
	if d.blob.GetEncodingType() != types.EncodingTypeJSON {
		return fmt.Errorf("unsupported encoding type %v", d.blob.GetEncodingType())
	}
	return json.Unmarshal(d.blob.Data, out)
}
