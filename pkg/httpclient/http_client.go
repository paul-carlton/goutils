package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

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

// reqResp hold information relating to an HTTPS request and response.
type reqResp struct {
	ReqResp
	ctx          context.Context
	logger       logr.Logger
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
	timeout *time.Duration, logger logr.Logger, client *http.Client, transport http.RoundTripper) (ReqResp, error) {
	if url == nil {
		return nil, ErrorInvalidURL
	}

	if logger == nil {
		logger = logging.NewLogger("httpClient", &zap.Options{})
	}

	if transport == nil {
		transport = tr
	}

	if client == nil {
		client = &http.Client{Transport: transport}
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
		logger:       logger,
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
				r.logger.Error(e, "failed to close response body")
			}
		}
	}
}

// HTTPreq creates an HTTP client and sends a request. The response is held in reqResp.RespText.
func (r *reqResp) HTTPreq() error { //nolint:funlen,gocyclo // ok
	var err error

	r.client.Timeout = *r.timeout

	// r.logger.V(logging.TraceLevel).Info("Request", "method", r.method, "url", r.url) //.

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

	for {
		r.resp, err = r.client.Do(httpReq) //nolint:bodyclose // ok
		if err != nil {                    //nolint:nestif // ok
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
					r.logger.Error(err, "server failed to respond", "url", r.url)

					continue
				}
			}

			return err
		}

		if err := r.getRespBody(); err != nil {
			return err
		}

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
			r.logger.Error(err, "failed to retrieve response body")

			return nil
		}
	}

	return r.respText
}

// RespCode is used to return the response code.
func (r *reqResp) RespCode() int {
	return r.resp.StatusCode
}
