package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/external"
)

var _ http.Handler = &Proxy{}

func TestProxyServeHTTP(t *testing.T) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		t.Fatal(err)
	}
	p := &Proxy{
		Config:       cfg,
		FunctionName: "proxy-test",
	}
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	t.Log(rec.Body)
}
