package proxy

import (
	"encoding/base64"
	"io"
	"io/ioutil"
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
	Instance InstanceContext `json:"instance"`
}

// InstanceContext contains the information to identify the ARN invoking the lambda
type InstanceContext struct {
	InstanceID string `json:"instance_id"`
	Hostname   string `json:"hostname"`
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

	// override the Host header
	header := req.Header
	if req.Host != "" && header.Get("Host") != req.Host {
		header = cloneHeader(header)
		header.Set("Host", req.Host)
	}

	h := make(map[string]string)
	for k, v := range header {
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
		MultiValueHeaders:               map[string][]string(header),
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
		h = make(http.Header, len(req.MultiValueHeaders))
		for k, vv := range req.MultiValueHeaders {
			for _, v := range vv {
				h.Add(k, v)
			}
		}
	} else {
		h = make(http.Header, len(req.Headers))
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

// Response returns http.Response.
func (resp *Response) Response() (*http.Response, error) {
	var header http.Header
	if len(resp.MultiValueHeaders) > 0 {
		header = make(http.Header, len(resp.MultiValueHeaders))
		for k, vv := range resp.MultiValueHeaders {
			for _, v := range vv {
				header.Add(k, v)
			}
		}
	} else {
		header = make(http.Header, len(resp.Headers))
		for k, v := range resp.Headers {
			header.Set(k, v)
		}
	}

	var body io.Reader = strings.NewReader(resp.Body)
	length := int64(len(resp.Body))
	if resp.IsBase64Encoded {
		body = base64.NewDecoder(base64.StdEncoding, body)
		length = int64(base64.StdEncoding.DecodedLen(len(resp.Body)))
	}

	return &http.Response{
		Status:        resp.StatusDescription,
		StatusCode:    resp.StatusCode,
		Proto:         "HTTP/1.0",
		ProtoMajor:    1,
		ProtoMinor:    0,
		Header:        header,
		Body:          ioutil.NopCloser(body),
		ContentLength: length,
	}, nil
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
