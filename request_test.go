package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRequestRequest(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		httpreq := httptest.NewRequest(http.MethodGet, "http://example.com/foo%20bar", nil)
		req, err := NewRequest(httpreq)
		if err != nil {
			t.Fatal(err)
		}
		want := &Request{
			HTTPMethod:                      http.MethodGet,
			Path:                            "/foo%20bar",
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
		if diff := cmp.Diff(req, want); diff != "" {
			t.Errorf("Request differs: (-got +want)\n%s", diff)
		}
	})

	t.Run("post", func(t *testing.T) {
		str := `{"hello":"world"}`
		r := strings.NewReader(str)
		httpreq := httptest.NewRequest(http.MethodPost, "http://example.com/foo%2fbar", r)
		httpreq.Header.Set("Content-Type", "application/json; charset=utf-8")
		req, err := NewRequest(httpreq)
		if err != nil {
			t.Fatal(err)
		}
		want := &Request{
			HTTPMethod:                      http.MethodPost,
			Path:                            "/foo%2fbar",
			QueryStringParameters:           map[string]string{},
			MultiValueQueryStringParameters: map[string][]string{},
			Headers: map[string]string{
				"Host":         "example.com",
				"Content-Type": "application/json; charset=utf-8",
			},
			MultiValueHeaders: map[string][]string{
				"Host":         []string{"example.com"},
				"Content-Type": []string{"application/json; charset=utf-8"},
			},
			IsBase64Encoded: false,
			Body:            `{"hello":"world"}`,
		}
		if diff := cmp.Diff(req, want); diff != "" {
			t.Errorf("Request differs: (-got +want)\n%s", diff)
		}
	})
}
