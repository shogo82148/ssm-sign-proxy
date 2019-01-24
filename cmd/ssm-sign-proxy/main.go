package main

import (
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	proxy "github.com/shogo82148/ssm-sign-proxy"
)

func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Fatal(err)
	}
	p := &proxy.Proxy{
		Config:       cfg,
		FunctionName: "proxy-test",
	}

	http.ListenAndServe(":8000", p)
}
