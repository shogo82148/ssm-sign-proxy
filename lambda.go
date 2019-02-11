package proxy

import (
	"context"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/ssmiface"
	"golang.org/x/sync/singleflight"
)

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
	Headers http.Header
}

// Sign adds authentication information to the request.
func (p *Parameter) Sign(req *http.Request) error {
	for k := range p.Headers {
		req.Header.Set(k, p.Headers.Get(k))
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
		for pager.Next() {
			resp := pager.CurrentPage()
			for _, param := range resp.Parameters {
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
				}
			}
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
