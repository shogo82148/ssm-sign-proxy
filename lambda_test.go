package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/external"
)

func TestLambdaHandle(t *testing.T) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		t.Fatal(err)
	}
	l := &Lambda{
		Config: cfg,
		Prefix: "development",
	}
	req := httptest.NewRequest(http.MethodGet, "https://api.mackerelio.com/api/v0/services", nil)
	r, err := NewRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := l.Handle(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp)
}
