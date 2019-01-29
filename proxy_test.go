package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/lambdaiface"
)

var _ http.Handler = &Proxy{}
var _ http.RoundTripper = &Proxy{}

type lambdaMock struct {
	lambdaiface.LambdaAPI
	input *lambda.InvokeInput
}

func TestProxyServeHTTP(t *testing.T) {
	l := &lambdaMock{}
	p := &Proxy{
		FunctionName: "proxy-test",
		scvlambda:    l,
	}
	httpreq := httptest.NewRequest(http.MethodGet, "https://example.com/foobar", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, httpreq)

	if aws.StringValue(l.input.FunctionName) != "proxy-test" {
		t.Errorf("want %s, got %s", "proxy-test", aws.StringValue(l.input.FunctionName))
	}
	var req Request
	if err := json.Unmarshal(l.input.Payload, &req); err != nil {
		t.Fatal(err)
	}
	want := Request{
		HTTPMethod: http.MethodGet,
		Path:       "/foobar",
		Headers: map[string]string{
			"Host":            "example.com",
			"X-Forwarded-For": "192.0.2.1",
		},
		MultiValueHeaders: map[string][]string{
			"Host":            []string{"example.com"},
			"X-Forwarded-For": []string{"192.0.2.1"},
		},
	}
	if !reflect.DeepEqual(req, want) {
		t.Errorf("want %#v, got %#v", want, req)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("want %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != `{"key":"value"}` {
		t.Errorf("want %s, got %s", `{"key":"value"}`, rec.Body.String())
	}
	if rec.HeaderMap.Get("Content-Type") != "application/json" {
		t.Errorf("want %s, got %s", "application/json", rec.HeaderMap.Get("Content-Type"))
	}
}

func (l *lambdaMock) InvokeRequest(input *lambda.InvokeInput) lambda.InvokeRequest {
	l.input = input
	out := &lambda.InvokeOutput{
		Payload: []byte(`{"statusCode":200,"headers":{"Content-Type":"application/json"},"body":"{\"key\":\"value\"}"}`),
	}
	return lambda.InvokeRequest{
		Request: &aws.Request{
			Data:        out,
			HTTPRequest: &http.Request{},
		},
		Input: input,
	}
}
