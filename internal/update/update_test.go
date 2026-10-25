package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			http.NotFound(w, r)
		case "/newer":
			_, _ = w.Write([]byte(`{"tag_name":"v0.4.0","html_url":"https://example.com/r"}`))
		case "/same":
			_, _ = w.Write([]byte(`{"tag_name":"0.3.0","html_url":"https://example.com/r"}`))
		default:
			_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://example.com/old"}`))
		}
	}))
	defer srv.Close()
	info, err := Check(context.Background(), "0.3.0", srv.URL+"/missing")
	if err != nil || info.Newer {
		t.Fatalf("404: %+v %v", info, err)
	}
	info, err = Check(context.Background(), "0.3.0", srv.URL+"/newer")
	if err != nil || !info.Newer || info.Latest != "0.4.0" || info.URL != "https://example.com/r" {
		t.Fatalf("newer: %+v %v", info, err)
	}
	info, err = Check(context.Background(), "0.3.0", srv.URL+"/same")
	if err != nil || info.Newer {
		t.Fatalf("same: %+v %v", info, err)
	}
	info, err = Check(context.Background(), "0.3.0", srv.URL+"/older")
	if err != nil || info.Newer {
		t.Fatalf("older: %+v %v", info, err)
	}
}
