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

	"github.com/nabancard/goutils/pkg/logging"
	"github.com/nabancard/goutils/pkg/miscutils"
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
	o         *miscutils.NewObjParams
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

func NewReqResp(objParams *miscutils.NewObjParams, timeout *time.Duration, client *http.Client, transport http.RoundTripper) (ReqResp, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	if transport == nil {
		transport = tr
	}

	if timeout == nil {
		timeout = &DefaultTimeout
	}

	if objParams.Ctx == nil {
		objParams.Ctx = context.Background()
	}

	if objParams.Log == nil {
		objParams.Log = logging.NewTextLoggerTo(objParams.LogOut)
	}

	r := reqResp{
		o:         objParams,
		transport: tr,
		client:    nil,
		timeout:   timeout,
		respText:  nil,
	}

	return &r, nil
}

// reqResp Methods

// CloseBody closes the response body.
func (r *reqResp) CloseBody() {
	logging.TraceCall()
	defer logging.TraceExit()

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
	logging.TraceCall()
	defer logging.TraceExit()

	var err error

	if url.Scheme == "https" {
		r.client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
		}
	} else {
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

	r.url = url

	var inputJSON io.ReadCloser

	if *r.method == Post { //nolint: nestif
		var jsonBytes []byte
		if b, ok := body.(string); ok {
			if logging.LogLevel <= logging.LevelTrace {
				r.o.Log.Log(r.o.Ctx, logging.LevelTrace, "body is a string, assuming it is valid json")
			}
			jsonBytes = []byte(b)
		} else {
			if logging.LogLevel <= logging.LevelTrace {
				r.o.Log.Log(r.o.Ctx, logging.LevelTrace, "body is not a string, marshalling to json")
			}
			jsonBytes, err = json.Marshal(r.body)
			if err != nil {
				return requestBodyError(err.Error())
			}
		}
		if logging.LogLevel <= logging.LevelTrace {
			fmt.Fprintf(r.o.LogOut, "body...\n%s\n", jsonBytes)
		}
		inputJSON = io.NopCloser(bytes.NewReader(jsonBytes))

		r.headerFields["Content-Type"] = "application/json"
		r.headerFields["Content-Length"] = fmt.Sprintf("%d", len(jsonBytes))
	}

	httpReq, err := http.NewRequestWithContext(r.o.Ctx, *r.method, r.url.String(), inputJSON)
	if err != nil {
		return readingResponseBodyError(err.Error())
	}

	for k, v := range r.headerFields {
		if len(v) > 0 {
			httpReq.Header.Set(k, v)
		}
	}

	r.o.Log.Debug("sending to", "url", url.String())

	retries := 30
	seconds := 1
	start := time.Now()
	for {
		r.resp, err = r.client.Do(httpReq) //nolint:bodyclose // ok
		if err != nil {                    //nolint:nestif // ok
			r.o.Log.Warn("failed to send request", slog.String("error", err.Error()))
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
					r.o.Log.Warn("server failed to respond", "url", r.url)
					r.o.Log.Warn("retrying")
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
	logging.TraceCall()
	defer logging.TraceExit()

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
	logging.TraceCall()
	defer logging.TraceExit()

	if r.respText == nil {
		if err := r.getRespBody(); err != nil {
			fmt.Printf("failed to retrieve response body: %s\n", err)
			return nil
		}
	}
	return r.respText
}

// RespCode is used to return the response code.
func (r *reqResp) RespCode() int {
	return r.resp.StatusCode
}
