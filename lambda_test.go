package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/ssmiface"
)

type ssmMock struct {
	ssmiface.SSMAPI
	input  *ssm.GetParametersByPathInput
	output *ssm.GetParametersByPathOutput
}

func TestLambdaHandle(t *testing.T) {
	t.Run("headers", func(t *testing.T) {
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			secret := req.Header.Get("Secret-Key")
			if secret != "very-secret" {
				t.Errorf("want %s, got %s", "very secret", secret)
				http.Error(w, "NG", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "ok")
		}))
		defer ts.Close()

		u, err := url.Parse(ts.URL)
		if err != nil {
			panic(err)
		}
		mock := &ssmMock{
			output: &ssm.GetParametersByPathOutput{
				Parameters: []ssm.Parameter{
					{
						Name:  aws.String("/development/" + u.Host + "/headers/secret-key"),
						Value: aws.String("very-secret"),
					},
				},
			},
		}
		l := &Lambda{
			Prefix: "development",
			Client: ts.Client(),
			svcssm: mock,
		}
		req := httptest.NewRequest(http.MethodGet, ts.URL, nil)
		r, err := NewRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := l.Handle(context.Background(), r)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Body != "ok" {
			t.Errorf("want %s, got %s", "ok", resp.Body)
		}

		if aws.StringValue(mock.input.Path) != "/development/"+u.Host {
			t.Errorf("want %s, goit %s", "/development/"+u.Host, aws.StringValue(mock.input.Path))
		}
	})
}

func (mock *ssmMock) GetParametersByPathRequest(input *ssm.GetParametersByPathInput) ssm.GetParametersByPathRequest {
	mock.input = input
	return ssm.GetParametersByPathRequest{
		Request: &aws.Request{
			Data:        mock.output,
			HTTPRequest: &http.Request{},
			Operation:   &aws.Operation{},
		},
		Input: input,
		Copy: func(*ssm.GetParametersByPathInput) ssm.GetParametersByPathRequest {
			return ssm.GetParametersByPathRequest{
				Request: &aws.Request{
					Data:        mock.output,
					HTTPRequest: &http.Request{},
					Operation:   &aws.Operation{},
				},
				Input: &ssm.GetParametersByPathInput{},
			}
		},
	}
}
