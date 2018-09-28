package tunnel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	for name, test := range map[string]struct {
		in  []Option
		out Options
	}{
		"default-options": {
			in: []Option{},
			out: Options{
				HaConnections:     HaConnectionsDefault,
				HeartbeatCount:    HeartbeatCountDefault,
				HeartbeatInterval: HeartbeatIntervalDefault,
				Retries:           RetriesDefault,
			},
		},
		"set-one-option": {
			in: []Option{
				Retries(100),
			},
			out: Options{
				HaConnections:     HaConnectionsDefault,
				HeartbeatCount:    HeartbeatCountDefault,
				HeartbeatInterval: HeartbeatIntervalDefault,
				Retries:           100,
			},
		},
		"set-all-options": {
			in: []Option{
				CompressionQuality(8),
				DisableChunkedEncoding(true),
				GracePeriod(100 * time.Millisecond),
				HaConnections(8),
				HeartbeatCount(100),
				HeartbeatInterval(100 * time.Millisecond),
				LbPool("test-lb"),
				Retries(100),
			},
			out: Options{
				CompressionQuality: 8,
				NoChunkedEncoding:  true,
				GracePeriod:        100 * time.Millisecond,
				HaConnections:      8,
				HeartbeatCount:     100,
				HeartbeatInterval:  100 * time.Millisecond,
				LbPool:             "test-lb",
				Retries:            100,
			},
		},
	} {
		out := CollectOptions(test.in)
		assert.Equalf(t, test.out, out, "test '%s' options mismatch", name)
	}
}
