package proxy

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
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

	h := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 {
			h[k] = v[len(v)-1]
		}
	}

	body, isBase64, err := readAll(req.Body)
	if err != nil {
		return nil, err
	}

	return &Request{
		HTTPMethod:                      req.Method,
		Path:                            req.URL.Path,
		QueryStringParameters:           q,
		MultiValueQueryStringParameters: map[string][]string(query),
		Headers:                         h,
		MultiValueHeaders:               map[string][]string(req.Header),
		IsBase64Encoded:                 isBase64,
		Body:                            body,
	}, nil
}

// Request returns http.Request.
func (req *Request) Request() (*http.Request, error) {
	// build the body
	var body io.Reader = strings.NewReader(req.Body)
	if req.IsBase64Encoded {
		body = base64.NewDecoder(base64.StdEncoding, body)
	}

	// build the query
	var q url.Values
	if len(req.MultiValueQueryStringParameters) > 0 {
		q = url.Values(req.MultiValueQueryStringParameters)
	} else {
		q = url.Values{}
		for k, v := range req.QueryStringParameters {
			q.Set(k, v)
		}
	}

	// build the headers
	var h http.Header
	if len(req.MultiValueHeaders) > 0 {
		h = http.Header(req.MultiValueHeaders)
	} else {
		h = http.Header{}
		for k, v := range req.Headers {
			h.Set(k, v)
		}
	}

	// build the target url
	host := h.Get("Host")
	u := url.URL{
		Scheme:   "https", // we don't care of http.
		Host:     host,
		Path:     req.Path,
		RawPath:  req.Path,
		RawQuery: q.Encode(),
	}

	// build the request
	httpreq, err := http.NewRequest(req.HTTPMethod, u.String(), body)
	if err != nil {
		return nil, err
	}
	httpreq.Header = h

	return httpreq, nil
}

// NewResponse returns new Response.
func NewResponse(resp *http.Response) (*Response, error) {
	body, isBase64, err := readAll(resp.Body)
	if err != nil {
		return nil, err
	}

	h := map[string]string{}
	for name := range resp.Header {
		h[name] = resp.Header.Get(name)
	}

	return &Response{
		StatusCode:        resp.StatusCode,
		StatusDescription: resp.Status,
		Headers:           h,
		MultiValueHeaders: map[string][]string(resp.Header),
		Body:              body,
		IsBase64Encoded:   isBase64,
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

func readAll(r io.Reader) (string, bool, error) {
	var body strings.Builder
	if _, err := io.Copy(&body, r); err != nil {
		return "", false, err
	}
	bodyString := body.String()
	if !utf8.ValidString(bodyString) {
		bodyString = base64.StdEncoding.EncodeToString([]byte(bodyString))
		return bodyString, true, nil
	}
	return bodyString, false, nil
}
