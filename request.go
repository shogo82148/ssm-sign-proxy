package proxy

import (
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Request contains data originating from the proxy.
type Request struct {
	HTTPMethod                      string              `json:"httpMethod"`
	Path                            string              `json:"path"`
	QueryStringParameters           map[string]string   `json:"queryStringParameters,omitempty"`
	MultiValueQueryStringParameters map[string][]string `json:"multiValueQueryStringParameters,omitempty"`
	Headers                         map[string]string   `json:"headers,omitempty"`
	MultiValueHeaders               map[string][]string `json:"multiValueHeaders,omitempty"`
	RequestContext                  RequestContext      `json:"requestContext"`
	IsBase64Encoded                 bool                `json:"isBase64Encoded"`
	Body                            string              `json:"body"`
}

// RequestContext contains the information to identify the instance invoking the lambda
type RequestContext struct {
	Instance InstanceContext `json:"elb"`
}

// InstanceContext contains the information to identify the ARN invoking the lambda
type InstanceContext struct {
	// TODO: implement this
}

// Response configures the response to be returned by the ALB Lambda target group for the request
type Response struct {
	StatusCode        int                 `json:"statusCode"`
	StatusDescription string              `json:"statusDescription"`
	Headers           map[string]string   `json:"headers"`
	MultiValueHeaders map[string][]string `json:"multiValueHeaders"`
	Body              string              `json:"body"`
	IsBase64Encoded   bool                `json:"isBase64Encoded"`
}

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

// NewRequest converts the request to AWS Lambda event.
func NewRequest(req *http.Request) (*Request, error) {
	// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/lambda-functions.html#multi-value-headers-request
	//
	query := req.URL.Query()
	q := make(map[string]string, len(query))
	for k, v := range query {
		if len(v) > 0 {
			q[k] = v[len(v)-1]
		}
	}

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
	h := make(map[string]string)
	for k, v := range header {
		if len(v) > 0 {
			h[k] = v[len(v)-1]
		}
	}

	var body strings.Builder
	if _, err := io.Copy(&body, req.Body); err != nil {
		return nil, err
	}
	var isBase64 bool
	bodyString := body.String()
	if !utf8.ValidString(bodyString) {
		bodyString = base64.StdEncoding.EncodeToString([]byte(bodyString))
		isBase64 = true
	}

	return &Request{
		HTTPMethod:                      req.Method,
		Path:                            req.URL.Path,
		QueryStringParameters:           q,
		MultiValueQueryStringParameters: map[string][]string(query),
		Headers:                         h,
		MultiValueHeaders:               map[string][]string(header),
		IsBase64Encoded:                 isBase64,
		Body:                            bodyString,
	}, nil
}

// WriteTo writes the response to w.
func (resp *Response) WriteTo(w http.ResponseWriter) error {
	// parse header
	header := w.Header()
	if len(resp.MultiValueHeaders) > 0 {
		for k, v := range resp.MultiValueHeaders {
			for _, vv := range v {
				header.Add(k, vv)
			}
		}
	} else {
		for k, v := range resp.Headers {
			header.Add(k, v)
		}
	}
	removeConnectionHeaders(header)
	for _, h := range hopHeaders {
		header.Del(h)
	}

	// parse the body
	var body []byte
	if resp.IsBase64Encoded {
		var err error
		body, err = base64.StdEncoding.DecodeString(resp.Body)
		if err != nil {
			return err
		}
	} else {
		body = []byte(resp.Body)
	}

	// parse status code
	header.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(resp.StatusCode)

	_, err := w.Write(body)
	return err
}

func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
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
