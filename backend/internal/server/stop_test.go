package server

import (
	"net/http"
	"testing"
)

func TestStopIsPrivateToUnixSocket(t *testing.T) {
	s, _ := newTestServer()
	stopped := 0
	s.Stop = func() { stopped++ }
	web := s.LocalHandler()
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		do(web, method, "/local/stop", "", map[string]string{
			"Cookie": "agentsterm_session=websession",
			"Origin": "http://127.0.0.1:7340",
		})
	}
	if stopped != 0 {
		t.Fatal("web browser stopped the daemon")
	}
	w := do(s.UnixHandler(nil), http.MethodPost, "/local/stop", "", nil)
	if w.Code != http.StatusNoContent || stopped != 1 {
		t.Fatalf("local stop: status=%d, calls=%d", w.Code, stopped)
	}
}
