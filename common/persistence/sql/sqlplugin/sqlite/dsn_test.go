package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/config"
)

func Test_buildDSN(t *testing.T) {
	for name, c := range map[string]struct {
		cfg  *config.SQL
		want string
	}{
		"empty": {
			cfg:  &config.SQL{},
			want: "file::memory:?_pragma=busy_timeout(10000)",
		},
		"database name only": {
			cfg: &config.SQL{
				DatabaseName: "cadence.db",
			},
			want: "file:cadence.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)",
		},
	} {
		t.Run(name, func(t *testing.T) {
			dsn := buildDSN(c.cfg)
			assert.Equal(t, c.want, dsn)
		})
	}
}

func Test_buildDSN_attrs(t *testing.T) {
	for name, c := range map[string]struct {
		cfg  *config.SQL
		want string
	}{
		"only connection attrs": {
			cfg: &config.SQL{
				ConnectAttributes: map[string]string{
					"_busy_timeout": "10000",
					"_FK":           "true",
				},
			},
			want: "file::memory:?_busy_timeout=10000&_fk=true&_pragma=busy_timeout(10000)",
		},
		"database name and connection attrs": {
			cfg: &config.SQL{
				DatabaseName: "cadence.db",
				ConnectAttributes: map[string]string{
					"cache1 ": "NONe ",
				},
			},
			want: "file:cadence.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&cache1=NONe",
		},
	} {
		t.Run(name, func(t *testing.T) {
			dsn := buildDSN(c.cfg)
			assert.Contains(t, c.want, dsn)
		})
	}
}
