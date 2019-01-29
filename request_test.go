package proxy

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestRequestRequest(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		httpreq := httptest.NewRequest(http.MethodGet, "http://example.com/foobar", nil)
		req, err := NewRequest(httpreq)
		if err != nil {
			t.Fatal(err)
		}
		want := &Request{
			HTTPMethod:                      http.MethodGet,
			Path:                            "/foobar",
			QueryStringParameters:           map[string]string{},
			MultiValueQueryStringParameters: map[string][]string{},
			Headers: map[string]string{
				"Host": "example.com",
			},
			MultiValueHeaders: map[string][]string{
				"Host": []string{"example.com"},
			},
			IsBase64Encoded: false,
			Body:            "",
		}
		if !reflect.DeepEqual(req, want) {
			t.Errorf("want %#v, got %#v", want, req)
		}
	})
}
