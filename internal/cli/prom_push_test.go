package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPushgatewayURL(t *testing.T) {
	u, err := pushgatewayURL("http://127.0.0.1:9091", "cursor-hooks", "wsl-1")
	if err != nil {
		t.Fatal(err)
	}
	want := "http://127.0.0.1:9091/metrics/job/cursor-hooks/instance/wsl-1"
	if u != want {
		t.Fatalf("got %q want %q", u, want)
	}
}

func TestRunPromPush_DryRun(t *testing.T) {
	// Exercise prom-push without network: dry-run only needs empty or temp metrics file.
	promPushFlags.dryRun = true
	promPushFlags.days = 1
	promPushFlags.withSmoke = false
	defer func() {
		promPushFlags.dryRun = false
	}()
	if err := runPromPush(nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestPromPush_HTTPServerAcceptsPost(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := pushgatewayURL(srv.URL, "j", "i")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{}
	resp, err := client.Post(u, "text/plain; version=0.0.4", strings.NewReader("ironclaw_test_metric 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if !strings.Contains(gotBody, "ironclaw_test_metric") {
		t.Fatalf("body %q", gotBody)
	}
}
