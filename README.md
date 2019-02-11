# ssm-sign-proxy
A proxy which signs requests using AWS System Manager Parameter Store.

## Motivation

There is a lot of HTTP APIs in the world, and some APIs require API tokens.
The aim of ssm-sign-proxy is to manage these API tokens intensively and centralizedly with [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-paramstore.html).


## Usage

### Deploy the AWS Serverless Application

Deploy the AWS Serverless Application from [AWS Serverless Application Repository](https://serverlessrepo.aws.amazon.com/applications/arn:aws:serverlessrepo:us-east-1:445285296882:applications~ssm-sign-proxy).

### Add API tokens to AWS System Manager Parameter Store

Add API tokens to AWS System Manager Parameter Store.
Here is an example for using [GitHub REST API v3](https://developer.github.com/v3/).

```
aws ssm put-parameter \
    --name "/api.github.com/headers/Authorization" \
    --value "token $YOUR_OAUTH_TOKEN_HERE" \
    --type SecureString
```

For more detail of parameters, see Supported Signing Methods section.

### Run the Proxy Server

Download from the binary from [Releases](https://github.com/shogo82148/ssm-sign-proxy/releases), or run `go get`.

```
$ go get github.com/shogo82148/ssm-sign-proxy/cmd/ssm-sign-proxy
```

Start the proxy with the `ssm-sign-proxy` command.

```
$ ssm-sign-proxy -function-name=ssm-sign-proxy-Proxy-XXXXXXXXXXXXX -addr=localhost:8000
```

Now, you can access the APIs registered in the Parameter Store without any authorize tokens.

```
http_proxy=localhost:8000 curl api.github.com/user/repos
```


## Supported Signing Methods

### Generic HTTP Headers

Use the following parameter names.

- `/{hostname}/headers/{header-name}`

Here is an example for GitHub API.

```
aws ssm put-parameter \
    --name "/api.github.com/headers/Authorization" \
    --value "token $YOUR_OAUTH_TOKEN_HERE" \
    --type SecureString
```

### Basic Authorization

Use the following parameter names.

- `/{hostname}/basic/username`
- `/{hostname}/basic/password`

Here is an example for GitHub API.

```
aws ssm put-parameter \
    --name "/api.github.com/basic/username" \
    --value "$YOUR_USER_NAME" \
    --type SecureString
aws ssm put-parameter \
    --name "/api.github.com/basic/password" \
    --value "$YOUR_PASSWORD" \
    --type SecureString
```

### Rewriting the Path of URL

Use the following parameter names.

- `/{hostname}/rewite/path`

Here is an example for [Slack Incoming Webhook](https://api.slack.com/incoming-webhooks).

```
aws ssm put-parameter \
    --name "/hooks.slack.com/rewite/path" \
    --value "/service/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX" \
    --type SecureString
```

### HTTP Queries

Use the following parameter names.

- `/{hostname}/queries/{name}`

Here is an example for GitHub API.

```
aws ssm put-parameter \
    --name "/api.github.com/queries/access_token" \
    --value "$YOUR_OAUTH_TOKEN_HERE" \
    --type SecureString
```


## License

MIT License

Copyright (c) 2019 Ichinose Shogo


## Related Works

- HTTP Proxy for Slack Incomming Webhook [cubicdaiya/slackboard](https://github.com/cubicdaiya/slackboard)
- Local Proxy for IRC [App::Ikachan](https://metacpan.org/pod/App::Ikachan)


## See Also

- [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-paramstore.html)
