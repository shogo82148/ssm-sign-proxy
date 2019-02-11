package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	proxy "github.com/shogo82148/ssm-sign-proxy"
)

var functionName, address string

func init() {
	flag.StringVar(&functionName, "function-name", "", "aws lambda function name")
	flag.StringVar(&address, "address", "localhost:8000", "address for listening")
}

func main() {
	flag.Parse()
	if functionName == "" {
		log.Fatal("-function-name is missing")
	}

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Fatal(err)
	}
	p := &proxy.Proxy{
		Config:       cfg,
		FunctionName: functionName,
	}

	http.ListenAndServe(address, p)
}
