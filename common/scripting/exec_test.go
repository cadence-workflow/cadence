package scripting

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBashExec(t *testing.T) {
	script := New()

	stdout, stderr, err := script.BashExec(context.Background(), "echo test")
	assert.Equal(t, "test\n", stdout)
	assert.Equal(t, "", stderr)
	assert.Nil(t, err)

	stdout, stderr, err = script.BashExec(context.Background(), "echo test 1>&2")
	assert.Equal(t, "test\n", stderr)
	assert.Equal(t, "", stdout)
	assert.Nil(t, err)

	stdout, stderr, err = script.BashExec(context.Background(), "false")
	assert.Equal(t, "", stderr)
	assert.Equal(t, "", stdout)
	assert.Error(t, err)

	tests := map[string]struct {
		args           string
		expectedStdout string
		expectedSterr  string
		expectedErr    bool
	}{
		"stdout": {
			args:           "echo test",
			expectedStdout: "test\n",
			expectedSterr:  "",
			expectedErr:    false,
		},
		"stderr": {
			args:           "echo test 1>&2",
			expectedStdout: "",
			expectedSterr:  "test\n",
			expectedErr:    false,
		},
		"error": {
			args:           "false",
			expectedStdout: "",
			expectedSterr:  "",
			expectedErr:    true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, err := script.BashExec(context.Background(), td.args)
			assert.Equal(t, td.expectedStdout, stdout)
			assert.Equal(t, td.expectedSterr, stderr)
			if td.expectedErr {
				assert.Error(t, err)
			}

			stdout, stderr, err = script.QuietBashExec(context.Background(), td.args)
			assert.Equal(t, td.expectedStdout, stdout)
			assert.Equal(t, td.expectedSterr, stderr)
			if td.expectedErr {
				assert.Error(t, err)
			}
		})
	}

}

func TestExec(t *testing.T) {
	script := New()

	tests := map[string]struct {
		bin            string
		args           []string
		expectedStdout string
		expectedSterr  string
		expectedErr    bool
	}{
		"stdout": {
			bin:            "echo",
			args:           []string{"test"},
			expectedStdout: "test\n",
			expectedSterr:  "",
			expectedErr:    false,
		},
		"error": {
			bin:            "false",
			args:           []string{""},
			expectedStdout: "",
			expectedSterr:  "",
			expectedErr:    true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, err := script.Exec(context.Background(), td.bin, td.args...)
			assert.Equal(t, td.expectedStdout, stdout)
			assert.Equal(t, td.expectedSterr, stderr)
			if td.expectedErr {
				assert.Error(t, err)
			}

			stdout, stderr, err = script.QuietExec(context.Background(), td.bin, td.args...)
			assert.Equal(t, td.expectedStdout, stdout)
			assert.Equal(t, td.expectedSterr, stderr)
			if td.expectedErr {
				assert.Error(t, err)
			}
		})
	}
}
