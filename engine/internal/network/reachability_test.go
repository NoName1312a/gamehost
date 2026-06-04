package network

import (
	"encoding/json"
	"testing"
)

func raw(m map[string]string) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for k, v := range m {
		out[k] = json.RawMessage(v)
	}
	return out
}

func TestInterpretReachability(t *testing.T) {
	cases := []struct {
		name        string
		nodes       map[string]string
		wantOpen    bool
		wantDecided bool
	}{
		{
			name:        "a node connected -> open",
			nodes:       map[string]string{"n1": `[{"error":"timed out"}]`, "n2": `[{"time":0.04,"address":"1.2.3.4"}]`},
			wantOpen:    true,
			wantDecided: true,
		},
		{
			name:        "all reporting nodes failed -> closed",
			nodes:       map[string]string{"n1": `[{"error":"timed out"}]`, "n2": `[{"error":"connection refused"}]`},
			wantOpen:    false,
			wantDecided: true,
		},
		{
			name:        "still pending (nulls) -> undecided",
			nodes:       map[string]string{"n1": `null`, "n2": `null`},
			wantOpen:    false,
			wantDecided: false,
		},
		{
			name:        "only one failure so far -> undecided (wait for more)",
			nodes:       map[string]string{"n1": `[{"error":"timed out"}]`, "n2": `null`},
			wantOpen:    false,
			wantDecided: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			open, decided := interpret(raw(c.nodes))
			if open != c.wantOpen || decided != c.wantDecided {
				t.Errorf("interpret() = (open=%v, decided=%v), want (open=%v, decided=%v)", open, decided, c.wantOpen, c.wantDecided)
			}
		})
	}
}
