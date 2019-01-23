package proxy

import (
	"context"
	"net/http"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// Lambda is a lambda function.
type Lambda struct {
	Config aws.Config

	mu sync.Mutex
}

// Handle hanles events of the AWS Lambda.
func (l *Lambda) Handle(ctx context.Context, req *Request) (*Response, error) {
	httpreq, err := req.Request()
	if err != nil {
		return nil, err
	}
	httpreq = httpreq.WithContext(ctx)

	resp, err := http.DefaultClient.Do(httpreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return NewResponse(resp)
}
