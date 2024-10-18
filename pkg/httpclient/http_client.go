package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	oneHundred = 100
	thirty     = 30
	ten        = 10
	one        = 1
)

var (
	ErrorInvalidURL         = errors.New("url is invalid")
	ErrorReadingRespBody    = errors.New("error reading response body")
	ErrorRequestFailed      = errors.New("error making request")
	ErrorRequestBodyInvalid = errors.New("failed to convert request body data to JSON")

	tr             *http.Transport //nolint:gochecknoglobals // ok
	DefaultTimeout time.Duration   //nolint:gochecknoglobals // ok
	Post           = "POST"        //nolint:gochecknoglobals // ok
	Delete         = "DELETE"      //nolint:gochecknoglobals // ok
	Get            = "GET"         //nolint:gochecknoglobals // ok
)

func readingResponseBodyError(msg string) error {
	return fmt.Errorf("%w: %s", ErrorReadingRespBody, msg)
}

func requestError(msg string) error {
	return fmt.Errorf("%w: %s", ErrorRequestFailed, msg)
}

func requestBodyError(msg string) error {
	return fmt.Errorf("%w: %s", ErrorRequestBodyInvalid, msg)
}

func init() { //nolint:gochecknoinits // ok
	tr = &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        oneHundred,
		MaxIdleConnsPerHost: oneHundred,
	}

	DefaultTimeout = time.Second * thirty
}

// Header is a type used to store header field name/value pairs when sending HTTPS requests.
type Header map[string]string

// reqResp hold information relating to an HTTPS request and response.
type reqResp struct {
	ReqResp
	ctx          context.Context
	client       *http.Client
	transport    *http.Transport
	url          *url.URL
	method       *string
	timeout      *time.Duration
	body         interface{}
	resp         *http.Response
	respText     *string
	headerFields Header
}

type ReqResp interface {
	HTTPreq() error
	getRespBody() error
	CloseBody()
	RespBody() *string
	ResponseCode() int
}

func NewReqResp(ctx context.Context, url *url.URL, method *string, body interface{}, header Header,
	timeout *time.Duration, client *http.Client, transport http.RoundTripper) (ReqResp, error) {
	if url == nil {
		return nil, ErrorInvalidURL
	}

	if transport == nil {
		transport = tr
	}

	if client == nil {
		if url.Scheme == "https" {
			fmt.Println("Creating TLS client")
			client = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						MinVersion: tls.VersionTLS12,
					},
				},
			}
		} else {
			fmt.Println("Creating HTTP client")
			client = &http.Client{Transport: transport}
		}
	}

	if header == nil {
		header = make(Header)
	}

	if method == nil {
		method = &Get
	}

	if timeout == nil {
		timeout = &DefaultTimeout
	}

	if ctx == nil {
		ctx = context.Background()
	}

	r := reqResp{
		ctx:          ctx,
		transport:    tr,
		client:       client,
		url:          url,
		method:       method,
		timeout:      timeout,
		body:         body,
		headerFields: header,
		respText:     nil,
	}

	return &r, nil
}

// reqResp Methods

// CloseBody closes the response body.
func (r *reqResp) CloseBody() {
	if r.resp != nil {
		if r.resp.Body != nil {
			e := r.resp.Body.Close()
			if e != nil {
				fmt.Printf("failed to close response body, %s", e)
			}
		}
	}
}

// HTTPreq creates an HTTP client and sends a request. The response is held in reqResp.RespText.
func (r *reqResp) HTTPreq() error { //nolint:funlen,gocyclo // ok
	var err error

	r.client.Timeout = *r.timeout

	fmt.Printf("Request, method: %s, url: %+v\n", *r.method, *r.url)

	var inputJSON io.ReadCloser

	if *r.method == Post {
		jsonBytes, e := json.Marshal(r.body)
		if e != nil {
			return requestBodyError(e.Error())
		}

		inputJSON = io.NopCloser(bytes.NewReader(jsonBytes))
	}

	httpReq, err := http.NewRequestWithContext(r.ctx, *r.method, r.url.String(), inputJSON)
	if err != nil {
		return readingResponseBodyError(err.Error())
	}

	for k, v := range r.headerFields {
		if len(v) > 0 {
			httpReq.Header.Set(k, v)
		}
	}

	retries := 30
	seconds := 1
	start := time.Now()
	fmt.Printf("Sending request at: %s\n", start)
	for {
		r.resp, err = r.client.Do(httpReq) //nolint:bodyclose // ok
		if err != nil {                    //nolint:nestif // ok
			fmt.Printf("Error sending request, %s\n", err)
			if strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "http2: no cached connection was available") ||
				strings.Contains(err.Error(), "net/http: TLS handshake timeout") ||
				strings.Contains(err.Error(), "i/o timeout") ||
				strings.Contains(err.Error(), "unexpected EOF") ||
				strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers") {
				time.Sleep(time.Second * time.Duration(int64(seconds)))

				retries--

				seconds += seconds

				if seconds > ten {
					seconds = one
				}

				if retries > 0 || time.Since(start) > *r.timeout {
					fmt.Printf("server failed to respond, url: %s\n", r.url)
					fmt.Printf("retrying")
					continue
				}
			}

			return err
		}
		fmt.Printf("sent request\n")
		if err := r.getRespBody(); err != nil {
			return err
		}

		fmt.Printf("got body of reply: %s\n", *r.respText)

		if r.resp.StatusCode == 200 || (r.resp.StatusCode == 201 && *r.method == Post) ||
			(r.resp.StatusCode == 204 && *r.method == Delete) {
			return nil
		}

		return requestError(fmt.Sprintf("failed: %s %s", r.resp.Status, *r.RespBody()))
	}
}

// getRespBody is used to obtain the response body as a string.
func (r *reqResp) getRespBody() error {
	defer r.resp.Body.Close()

	data, err := io.ReadAll(r.resp.Body)
	if err != nil {
		return readingResponseBodyError(err.Error())
	}

	strData := string(data)
	r.respText = &strData

	return nil
}

// RespBody is used to return the response body as a string.
func (r *reqResp) RespBody() *string {
	if r.respText == nil {
		if err := r.getRespBody(); err != nil {
			fmt.Printf("failed to retrieve response body: %s\n", err)
			return nil
		}
	}
	fmt.Printf("reqResp...\n%+v", *r)
	return r.respText
}

// RespCode is used to return the response code.
func (r *reqResp) RespCode() int {
	return r.resp.StatusCode
}
