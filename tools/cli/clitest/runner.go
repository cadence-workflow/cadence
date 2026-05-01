package clitest

import (
	"testing"

	"github.com/flynn/go-shlex"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

// RunCommandLine runs cmdline (a whole line like you'd type it in Shell)
func RunCommandLine(t *testing.T, app *cli.App, cmdline string) error {
	t.Helper()

	args, err := shlex.Split(cmdline)
	require.NoError(t, err)
	return app.Run(args)
}
