package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitlab.snapp.ir/snappcloud/health_exporter/internal/config"
)

func TestBuildClientH2C(t *testing.T) {
	t.Parallel()

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Proto))
	}))

	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)
	srv.Config.Protocols = &protocols
	srv.Start()
	t.Cleanup(srv.Close)

	client := buildClient(config.HTTPTarget{
		URL:        srv.URL,
		Timeout:    5 * time.Second,
		H2cEnabled: true,
	})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.ProtoMajor != 2 {
		t.Fatalf("expected an h2c response, got %s", resp.Proto)
	}
}
