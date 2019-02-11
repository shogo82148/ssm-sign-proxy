package proxy

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/lambdaiface"
)

// Hop-by-hop headers. These are removed when sent to the backend.
// As of RFC 7230, hop-by-hop headers are required to appear in the
// Connection header field. These are the headers defined by the
// obsoleted RFC 2616 (section 13.5.1) and are used for backward
// compatibility.
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonicalized version of "TE"
	"Trailer", // not Trailers per URL above; https://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

// Proxy is a proxy which signs requests using AWS System Manager Parameter Store.
type Proxy struct {
	Config       aws.Config
	FunctionName string
	ErrorHandler func(http.ResponseWriter, *http.Request, error)

	mu        sync.Mutex
	scvlambda lambdaiface.LambdaAPI
}

func (p *Proxy) lambda() lambdaiface.LambdaAPI {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.scvlambda == nil {
		p.scvlambda = lambda.New(p.Config)
	}
	return p.scvlambda
}

func (p *Proxy) errorHandler() func(http.ResponseWriter, *http.Request, error) {
	if p.ErrorHandler != nil {
		return p.ErrorHandler
	}
	return defaultErrorHandler
}

func defaultErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
	log.Println(err)
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

	resp, err := p.roundTrip(req2)
	if err != nil {
		p.errorHandler()(w, req, err)
		return
	}
	removeConnectionHeaders(http.Header(resp.MultiValueHeaders))
	for _, h := range hopHeaders {
		http.Header(resp.MultiValueHeaders).Del(h)
	}
	resp.WriteTo(w)
}

// RoundTrip implements the http.RoundTripper interface.
func (p *Proxy) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := p.roundTrip(req)
	if err != nil {
		return nil, err
	}
	return resp.Response()
}

func (p *Proxy) roundTrip(req *http.Request) (*Response, error) {
	// parse request
	request, err := NewRequest(req)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// invoke the lambda function
	r := p.lambda().InvokeRequest(&lambda.InvokeInput{
		FunctionName: aws.String(p.FunctionName),
		Payload:      payload,
	})
	r.SetContext(req.Context())
	response, err := r.Send()
	if err != nil {
		return nil, err
	}
	if response.FunctionError != nil {
		return nil, parseError(response.Payload)
	}

	// build the response
	var resp Response
	if err := json.Unmarshal(response.Payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// removeConnectionHeaders removes hop-by-hop headers listed in the "Connection" header of h.
// See RFC 7230, section 6.1
func removeConnectionHeaders(h http.Header) {
	if c := h.Get("Connection"); c != "" {
		for _, f := range strings.Split(c, ",") {
			if f = strings.TrimSpace(f); f != "" {
				h.Del(f)
			}
		}
	}
}
