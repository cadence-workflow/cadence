package clitest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestRunCommandLine(t *testing.T) {
	var output string

	testApp := &cli.App{
		Name: "devmngr",
		Commands: []*cli.Command{
			{
				Name: "upgrade",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "device"},
					&cli.BoolFlag{Name: "enabled"},
					&cli.StringFlag{Name: "reason"},
				},
				Action: func(c *cli.Context) error {
					output = fmt.Sprintf(
						"upgrading device: \"%s\"; enabled: %v; reason: \"%s\"",
						c.String("device"),
						c.Bool("enabled"),
						c.String("reason"),
					)
					return nil
				},
			},
		},
	}

	err := RunCommandLine(
		t,
		testApp,
		"devmngr upgrade --device video-card --enabled --reason \"Doom: The Dark Ages is released\"",
	)
	require.NoError(t, err)
	assert.Equal(t,
		"upgrading device: \"video-card\"; enabled: true; reason: \"Doom: The Dark Ages is released\"",
		output,
	)
}
