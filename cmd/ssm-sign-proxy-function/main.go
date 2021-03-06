package main

import (
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	proxy "github.com/shogo82148/ssm-sign-proxy"
)

func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Fatal(err)
	}
	l := &proxy.Lambda{
		Config: cfg,
		Prefix: os.Getenv("SSM_SIGN_PROXY_PREFIX"),
	}

	lambda.Start(l.Handle)
}
