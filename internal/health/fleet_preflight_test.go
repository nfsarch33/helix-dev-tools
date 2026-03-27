package health

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestRunFleetPreflight_OK(t *testing.T) {
	ctx := context.Background()
	get := func(_ context.Context, _ *http.Client, _ string) (int, error) {
		return 200, nil
	}
	res := RunFleetPreflight(ctx, FleetPreflightOptions{
		HTTPGet: get,
	})
	if !res.OK() {
		t.Fatalf("expected OK, got %+v", res)
	}
}

func TestRunFleetPreflight_HTTPError(t *testing.T) {
	ctx := context.Background()
	get := func(_ context.Context, _ *http.Client, url string) (int, error) {
		if url == DefaultFleetDRLHealthURL {
			return 0, fmt.Errorf("connection refused")
		}
		return 200, nil
	}
	res := RunFleetPreflight(ctx, FleetPreflightOptions{HTTPGet: get})
	if res.OK() {
		t.Fatal("expected failure")
	}
	if !res.AnyFailed {
		t.Fatal("expected AnyFailed")
	}
}

func TestRunFleetPreflight_HTTP5xx(t *testing.T) {
	ctx := context.Background()
	n := 0
	get := func(_ context.Context, _ *http.Client, _ string) (int, error) {
		n++
		if n == 1 {
			return 503, nil
		}
		return 200, nil
	}
	res := RunFleetPreflight(ctx, FleetPreflightOptions{HTTPGet: get})
	if res.OK() {
		t.Fatal("expected failure on 503")
	}
}

func TestRunFleetPreflight_ComposeOK(t *testing.T) {
	ctx := context.Background()
	get := func(_ context.Context, _ *http.Client, _ string) (int, error) {
		return 200, nil
	}
	compose := func(_ context.Context, _ string) (string, error) {
		return "NAME\nfoo", nil
	}
	res := RunFleetPreflight(ctx, FleetPreflightOptions{
		HTTPGet:   get,
		ComposePS: compose,
		ComposeDir: "/tmp/fake-repo",
	})
	if !res.OK() || !res.ComposeRan || !res.ComposeOK {
		t.Fatalf("expected compose ok, got %+v", res)
	}
}

func TestRunFleetPreflight_ComposeFail(t *testing.T) {
	ctx := context.Background()
	get := func(_ context.Context, _ *http.Client, _ string) (int, error) {
		return 200, nil
	}
	compose := func(_ context.Context, _ string) (string, error) {
		return "", fmt.Errorf("no docker")
	}
	res := RunFleetPreflight(ctx, FleetPreflightOptions{
		HTTPGet:   get,
		ComposePS: compose,
		ComposeDir: "/tmp/fake-repo",
	})
	if res.OK() {
		t.Fatal("expected compose failure")
	}
}
