package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/ssmiface"
	"golang.org/x/sync/singleflight"
)

var errParamNotFound = errors.New("proxy: parameters not found")

// Lambda is a lambda function.
type Lambda struct {
	Config aws.Config
	Prefix string
	Client *http.Client

	group  singleflight.Group
	mu     sync.RWMutex
	cache  map[string]*Parameter
	svcssm ssmiface.SSMAPI
}

func (l *Lambda) ssm() ssmiface.SSMAPI {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.svcssm == nil {
		l.svcssm = ssm.New(l.Config)
	}
	return l.svcssm
}

func (l *Lambda) client() *http.Client {
	if l.Client != nil {
		return l.Client
	}
	return http.DefaultClient
}

// Handle hanles events of the AWS Lambda.
func (l *Lambda) Handle(ctx context.Context, req *Request) (*Response, error) {
	httpreq, err := req.Request()
	if err != nil {
		return nil, err
	}
	httpreq = httpreq.WithContext(ctx)

	param, err := l.getParam(ctx, httpreq.Header.Get("Host"))
	if err != nil {
		if err == errParamNotFound {
			return &Response{
				StatusCode: http.StatusProxyAuthRequired,
				Headers: map[string]string{
					"Content-Type": "text/plain; charset=utf-8",
				},
				Body: "any parameters for signing is not found in AWS System Manager Parameter Store\n",
			}, nil
		}
		return nil, err
	}
	if err := param.Sign(httpreq); err != nil {
		return nil, err
	}

	resp, err := l.client().Do(httpreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return NewResponse(resp)
}

// Parameter is parameter for signing.
type Parameter struct {
	// general http headers
	Headers http.Header

	// basic authorization
	User     string
	Password string

	// rewrite url
	Path string

	// general queries
	Queries url.Values
}

// Sign adds authentication information to the request.
func (p *Parameter) Sign(req *http.Request) error {
	for k := range p.Headers {
		req.Header.Set(k, p.Headers.Get(k))
	}
	if p.User != "" {
		req.SetBasicAuth(p.User, p.Password)
	}
	q := req.URL.Query()
	if len(p.Queries) > 0 {
		for k := range p.Queries {
			q.Set(k, p.Queries.Get(k))
		}
		req.URL.RawQuery = q.Encode()
	}
	if p.Path != "" {
		req.URL.RawPath = p.Path
		req.URL.Path = p.Path
	}
	return nil
}

func (l *Lambda) getParam(ctx context.Context, host string) (*Parameter, error) {
	host = strings.ToLower(host)
	result := l.group.DoChan(host, func() (interface{}, error) {
		// search from the cache.
		l.mu.RLock()
		if l.cache != nil && l.cache[host] != nil {
			l.mu.RUnlock()
			return l.cache[host], nil
		}
		l.mu.RUnlock()

		// get from AWS SSM Parameter Store.
		parameter := &Parameter{}
		base := path.Join("/", l.Prefix, host)

		svc := l.ssm()
		req := svc.GetParametersByPathRequest(&ssm.GetParametersByPathInput{
			Path:           aws.String(base),
			Recursive:      aws.Bool(true),
			WithDecryption: aws.Bool(true),
		})
		req.SetContext(context.Background())
		pager := req.Paginate()
		cnt := 0
		for pager.Next() {
			resp := pager.CurrentPage()
			for _, param := range resp.Parameters {
				cnt++
				name := strings.TrimPrefix(aws.StringValue(param.Name), base+"/")
				name = strings.TrimSuffix(name, "/")
				idx := strings.IndexByte(name, '/')
				if idx < 0 {
					continue
				}
				typ := name[:idx]
				name = name[idx+1:]
				switch typ {
				case "headers":
					if parameter.Headers == nil {
						parameter.Headers = http.Header{}
					}
					parameter.Headers.Set(name, aws.StringValue(param.Value))
				case "basic":
					switch name {
					case "username":
						parameter.User = aws.StringValue(param.Value)
					case "password":
						parameter.Password = aws.StringValue(param.Value)
					}
				case "rewrite":
					switch name {
					case "path":
						parameter.Path = aws.StringValue(param.Value)
					}
				case "queries":
					if parameter.Queries == nil {
						parameter.Queries = url.Values{}
					}
					parameter.Queries.Set(name, aws.StringValue(param.Value))
				}
			}
		}
		if cnt == 0 {
			return nil, errParamNotFound
		}

		// set to the cache.
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.cache == nil {
			l.cache = make(map[string]*Parameter)
		}
		l.cache[host] = parameter
		return parameter, nil
	})
	select {
	case r := <-result:
		if r.Err != nil {
			return nil, r.Err
		}
		return r.Val.(*Parameter), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
