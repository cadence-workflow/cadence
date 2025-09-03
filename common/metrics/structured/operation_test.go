package structured

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTags(t *testing.T) {
	// intentionally shared to increase concurrency
	e := NewTestEmitter(t, nil)
	t.Run("operation", func(t *testing.T) {
		t.Parallel()
		tags := e.getTags(OperationTags{Operation: "test"})
		assert.Equal(t, Tags{map[string]string{
			"operation": "test",
		}}, tags)
	})
	t.Run("dynamic operation", func(t *testing.T) {
		t.Parallel()
		tags := e.getTags(DynamicOperationTags{DynamicOperation: struct{}{}})
		assert.Empty(t, tags)
	})
	t.Run("embedded operations", func(t *testing.T) {
		t.Parallel()
		type x struct {
			OperationTags

			Field string `tag:"field"`
		}
		tags := e.getTags(x{
			OperationTags: OperationTags{Operation: "test"},
			Field:         "value",
		})
		assert.Equal(t, Tags{map[string]string{
			"field":     "value",
			"operation": "test",
		}}, tags)
	})
	t.Run("deeply embedded operations", func(t *testing.T) {
		t.Parallel()
		type X struct { // must be public so it can be accessed through Y
			OperationTags
			Field string `tag:"field"`
		}
		type Y struct { // must be public so it can be accessed through Z
			TopField string `tag:"top_field"`
			X
		}
		type z struct {
			Reserved struct{} `tag:"reserved"`
			Y
			Another int `tag:"another"`
		}
		tags := e.getTags(z{
			Y: Y{
				TopField: "top",
				X: X{
					OperationTags: OperationTags{
						Operation: "test",
					},
					Field: "value",
				},
			},
			Another: 42,
		})
		assert.Equal(t, Tags{map[string]string{
			"top_field": "top",
			"another":   "42",
			"operation": "test",
			"field":     "value",
		}}, tags)
	})
	t.Run("private with getter", func(t *testing.T) {
		t.Parallel()
		tags := e.getTags(privategetter{"anything"})
		assert.Equal(t, Tags{map[string]string{
			"private": "anything_via_getter",
		}}, tags)
	})
	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		type q struct {
			private string `tag:"private"`
		}
		assert.PanicsWithValue(t,
			"field private on type github.com/uber/cadence/common/metrics/structured.q "+
				"must be public or have a getter func on the parent struct",
			func() {
				tags := e.getTags(q{"anything"})
				t.Errorf("%#v", tags) // unreachable if working correctly
			},
		)
	})

}

type privategetter struct {
	private string `tag:"private" getter:"GetIt"`
}

func (p privategetter) GetIt() string {
	return p.private + "_via_getter"
}
