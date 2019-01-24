package proxy

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
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
	header := cloneHeader(req.Header)
	removeConnectionHeaders(header)
	// Remove hop-by-hop headers to the backend. Especially
	// important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	for _, h := range hopHeaders {
		hv := header.Get(h)
		if hv == "" {
			continue
		}
		if h == "Te" && hv == "trailers" {
			// Issue 21096: tell backend applications that
			// care about trailer support that we support
			// trailers. (We do, but we don't go out of
			// our way to advertise that unless the
			// incoming client request thought it was
			// worth mentioning)
			continue
		}
		header.Del(h)
	}

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		header.Set("X-Forwarded-For", clientIP)
	}
	req2 := &http.Request{}
	*req2 = *req
	req2.Header = header

	if host := req2.URL.Host; host != "" {
		// work as a forward proxy
		req2.Host = host
		header.Set("Host", host)
	}

	// parse request
	request, err := NewRequest(req2)
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
