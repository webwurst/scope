package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/weaveworks/scope/test"
)

func TestAPIOriginHost(t *testing.T) {
	ts := httptest.NewServer(Router(StaticReport{}))
	defer ts.Close()

	is404(t, ts, "/api/origin/foobar")
	is404(t, ts, "/api/origin/host/foobar")

	{
		// Origin
		body := getRawJSON(t, ts, fmt.Sprintf("/api/origin/host/%s", test.ServerHostNodeID))
		var h OriginHost
		if err := json.Unmarshal(body, &h); err != nil {
			t.Fatal(err)
		}
		if want, have := test.ServerHostOS, h.OS; want != have {
			t.Errorf("want %q, have %q", want, have)
		}
		if want, have := test.ServerHostLoad, h.Load; want != have {
			t.Errorf("want %q, have %q", want, have)
		}
	}
}
