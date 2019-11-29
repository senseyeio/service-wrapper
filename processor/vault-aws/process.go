package vaultaws

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/pkg/errors"
)

const (
	VAULT_AWS_LOGIN = "/v1/auth/aws/login"
	CONSUL_BINARY   = "/bin/envconsul"
)

type VaultAWS struct {
	Role    string `json:"role"`
	Method  string `json:"iam_http_request_method"`
	Url     string `json:"iam_request_url"`
	Headers string `json:"iam_request_headers"`
	Body    string `json:"iam_request_body"`
}

const (
	DEFAULT_TEMPLATE = ``
)

type config struct {
	Vault    string `envconfig:"VAULT_ADDR"`
	Role     string `envconfig:"VAULT_ROLE"`
	Config   string `envconfig:"VAULT_SERVICE_CONFIG"`
	Filename string `envconfig:"VAULT_SERVICE_FILE"`
	// AWS Credentials
	AWSPath   string `envconfig:"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"`
	AWSAccess string `envconfig:"AWS_ACCESS_KEY_ID"`
	AWSSecret string `envconfig:"AWS_SECRET_ACCESS_KEY"`
}

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type awsAuth struct {
	vaultEndpoint   *url.URL
	role            string
	httpc           Doer
	serviceConfig   string
	serviceFilename string
}

func New(httpc Doer) *awsAuth {
	return &awsAuth{
		httpc: httpc,
	}
}

func (a *awsAuth) Config() *config {
	return &config{}
}

func (a *awsAuth) Init(c interface{}) (bool, error) {

	var cfg *config

	cfg, ok := c.(*config)
	if !ok {
		return false, errors.New("bad config")
	}

	validAWSCredentials := cfg.AWSPath != "" || (cfg.AWSAccess != "" && cfg.AWSSecret != "")
	if cfg.Vault == "" || !validAWSCredentials {
		return false, nil
	}

	vaultEndpoint, urlErr := url.Parse(cfg.Vault)
	if urlErr != nil {
		return false, errors.Wrap(urlErr, "invalid url for VAULT_ADDR")
	}

	if vaultEndpoint.Hostname() == "" {
		return false, errors.New("missing hostname in VAULT_ADDR")
	}
	if cfg.Role == "" {
		return false, errors.New("missing role for Vault AWS Auth")
	}

	if cfg.Config != "" && cfg.Filename == "" {
		return false, errors.New("missing filename for Vault AWS Auth config")
	}

	a.vaultEndpoint = vaultEndpoint
	a.role = cfg.Role
	a.serviceFilename = cfg.Filename
	a.serviceConfig = cfg.Config

	return true, nil
}

type vaultResponse struct {
	Auth struct {
		ClientToken string `json:"client_token"`
	} `json:"auth"`
}

func doPost(httpc Doer, url string, request, response interface{}) (int, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return 0, errors.Wrap(err, "Error building vault request")
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Add("Content-Type", "application/json")

	resp, err := httpc.Do(req)

	if err != nil {
		return 0, errors.Wrap(err, "Failed Vault AWS Auth login")
	}

	responseBody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err != nil {
		return resp.StatusCode, errors.Wrap(err, "Error reading body response")
	}

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("Vault returned status code %d body: %s", resp.StatusCode, string(responseBody))
	}

	err = json.Unmarshal(responseBody, response)
	if err != nil {
		return resp.StatusCode, errors.Wrap(err, "Error decoding json response")
	}

	return resp.StatusCode, nil
}

func (a *awsAuth) Apply(oldArgs []string, oldEnv []string) ([]string, []string, error) {
	iamRequest, err := a.buildRequest()

	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to build Vault AWS Auth request")
	}

	url := fmt.Sprintf("%s%s", a.vaultEndpoint, VAULT_AWS_LOGIN)

	var loginResponse vaultResponse

	_, err = doPost(a.httpc, url, &iamRequest, &loginResponse)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to make Vault AWS Auth request")
	}

	// extract loginReponse.Auth.ClientToken
	token := loginResponse.Auth.ClientToken
	if token == "" {
		return nil, nil, errors.New("Vault token is invalid")
	}

	if a.serviceConfig != "" {
		err := a.writeConfig(oldEnv)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Vault AWS Auth config failed")
		}
	}

	// If we use response wrapping its ok for the vault-token to be visible like this
	// the alternative is to write out a new service.hcl with the values in
	// in the file. Most likely to /run and make this a tmpfs

	args := []string{CONSUL_BINARY}
	if a.serviceFilename != "" {
		cf := fmt.Sprintf("-config=%s", a.serviceFilename)
		args = append(args, cf)
	}

	va := fmt.Sprintf("-vault-addr=%s", a.vaultEndpoint.String())
	vt := fmt.Sprintf("-vault-token=%s", token)

	args = append(args, va, vt)

	args = append(args, oldArgs...)

	return args, oldEnv, nil
}

// Use the aws-sdk to pull in suitable aws credentials that will be
// passed to Vault for authentication via its AWS Auth method (using IAM)
// These are expected to be via the AWS_CONTAINER_CREDENTIALS_RELATIVE_URI
// endpoint but we let the AWS sdk build the request
func (a *awsAuth) buildRequest() (*VaultAWS, error) {
	sess := session.Must(session.NewSession())

	client := sts.New(sess)

	input := &sts.GetCallerIdentityInput{}
	req, _ := client.GetCallerIdentityRequest(input)

	headers := req.HTTPRequest.Header
	headers.Add("X-Vault-AWS-IAM-Server-ID", a.vaultEndpoint.Hostname())
	headers.Del("Host")

	req.Sign()

	body, err := ioutil.ReadAll(req.GetBody())
	if err != nil {
		return nil, errors.Wrap(err, "error extracting caller identity request body")
	}

	jsonHeaders, _ := json.Marshal(headers)

	encode := func(data []byte) string {
		return b64.StdEncoding.EncodeToString(data)
	}

	return &VaultAWS{
		Role:    a.role,
		Method:  req.HTTPRequest.Method,
		Url:     encode([]byte(req.HTTPRequest.URL.String())),
		Headers: encode(jsonHeaders),
		Body:    encode(body),
	}, nil
}

func environAsMap(data []string) map[string]string {
	env := map[string]string{}
	for _, entry := range data {
		parts := strings.SplitN(entry, "=", 2)
		env[parts[0]] = parts[1]
	}
	return env
}

// Write service config
func (a *awsAuth) writeConfig(env []string) error {

	tmpl, err := template.New("service").Parse(a.serviceConfig)
	if err != nil {
		return err
	}

	file, err := os.Create(a.serviceFilename)
	if err != nil {
		return err
	}
	defer file.Close()

	err = tmpl.Execute(file, environAsMap(env))
	if err != nil {
		return err
	}

	return nil
}
