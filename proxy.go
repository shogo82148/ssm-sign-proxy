package proxy

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// Proxy is a proxy which signs requests using AWS System Manager Parameter Store.
type Proxy struct {
	Config       aws.Config
	FunctionName string

	mu        sync.Mutex
	scvlambda *lambda.Lambda
}

func (p *Proxy) lambda() *lambda.Lambda {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.scvlambda == nil {
		p.scvlambda = lambda.New(p.Config)
	}
	return p.scvlambda
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// parse request
	request, err := NewRequest(req)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	payload, err := json.Marshal(request)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// invoke the lambda function
	r := p.lambda().InvokeRequest(&lambda.InvokeInput{
		FunctionName: aws.String(p.FunctionName),
		Payload:      payload,
	})
	r.SetContext(req.Context())
	response, err := r.Send()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// build the response
	var resp Response
	if err := json.Unmarshal(response.Payload, &resp); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	resp.WriteTo(w)
}
