package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/paul-carlton/goutils/pkg/logging"
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

// reqResp hold information relating to an HTTP(S) request and response.
type reqResp struct {
	ReqResp
	ctx       context.Context
	log       *slog.Logger
	client    *http.Client
	transport *http.Transport
	timeout   *time.Duration

	url          *url.URL
	method       *string
	body         interface{}
	headerFields Header

	resp     *http.Response
	respText *string
}

type ReqResp interface {
	HTTPreq(method *string, url *url.URL, body interface{}, header Header) error
	getRespBody() error
	CloseBody()
	RespBody() *string
	RespCode() int
}

func NewReqResp(ctx context.Context, logger *slog.Logger,
	timeout *time.Duration, client *http.Client, transport http.RoundTripper) (ReqResp, error) {
	if transport == nil {
		transport = tr
	}

	if timeout == nil {
		timeout = &DefaultTimeout
	}

	if ctx == nil {
		ctx = context.Background()
	}

	r := reqResp{
		ctx:       ctx,
		log:       logger,
		transport: tr,
		client:    client,
		timeout:   timeout,
		respText:  nil,
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
func (r *reqResp) HTTPreq(method *string, url *url.URL, body interface{}, header Header) error { //nolint:funlen,gocyclo,gocognit // ok
	var err error

	if url.Scheme == "https" {
		fmt.Println("Creating TLS client")
		r.client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
		}
	} else {
		fmt.Println("Creating HTTP client")
		r.client = &http.Client{Transport: r.transport}
	}

	if header == nil {
		header = make(Header)
	}
	r.headerFields = header

	if method == nil {
		method = &Get
	}
	r.method = method

	r.client.Timeout = *r.timeout

	r.log.Debug("Request", "method", *r.method, "url", r.url.String())

	var inputJSON io.ReadCloser

	if *r.method == Post {
		var jsonBytes []byte
		if b, ok := body.(string); ok {
			jsonBytes = []byte(b)
		} else {
			jsonBytes, err = json.Marshal(r.body)
			if err != nil {
				return requestBodyError(err.Error())
			}
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
	r.log.Log(r.ctx, logging.LevelTrace, "Sending request", slog.Time("start", start))
	for {
		r.resp, err = r.client.Do(httpReq) //nolint:bodyclose // ok
		if err != nil {                    //nolint:nestif // ok
			r.log.Warn("failed to send request", slog.String("error", err.Error()))
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
					r.log.Warn("server failed to respond", "url", r.url)
					r.log.Warn("retrying")
					continue
				}
			}

			return err
		}
		r.log.Log(r.ctx, logging.LevelTrace, "sent request")
		if err := r.getRespBody(); err != nil {
			return err
		}

		r.log.Log(r.ctx, logging.LevelTrace, "got reply", slog.Int("code", r.resp.StatusCode), "reply", *r.respText)

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
	if strData == "null" {
		strData = ""
	}
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
