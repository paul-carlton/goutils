package httpserver

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	AppJSON     = "application/json"
	ContentType = "Content-Type"
	ten         = 10
	five        = 5
	thirty      = 30
	three       = 3
)

var ErrorReadingBody = errors.New("error reading body")

func readingBodyError(msg string) error {
	return fmt.Errorf("%w: %s", ErrorReadingBody, msg)
}

// HandlerHTTP is a type defining a structure used to manage HTTP server setup and request handling.
type HandlerHTTP struct {
	Address    string
	ListenPort int
	C          chan string
	Mux        MuxHTTP
	Server     *http.Server
}

// MuxHTTP is a type defining a map of pathnames to functions for handling incoming http requests.
type MuxHTTP map[string]func(http.ResponseWriter, *http.Request) (int, string)

// GetServer ... .
func (handler *HandlerHTTP) GetServer() {
}

// ServeHTTP serves incomming HTTP requests.
func (handler *HandlerHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infof("Incoming request: %s", r.URL.String())
	reqLine := r.URL.String()
	split := strings.Index(reqLine, "?")

	var ThePath string

	if split > 0 {
		ThePath = reqLine[:split]
	} else {
		ThePath = reqLine
	}

	if h, ok := handler.Mux[ThePath]; ok {
		code, msg := h(w, r)
		if !expectedHTTPstatus(code, r.Method) {
			http.Error(w, msg, code)

			return
		}

		return
	}

	http.Error(w, fmt.Sprintf("unrecognised request %s", ThePath), http.StatusBadRequest)
}

func (handler *HandlerHTTP) Start(_ *MuxHTTP) {
	go serveHTTP(*handler)
}

func (handler *HandlerHTTP) Shutdown() {
	handler.C <- "0"
}

func expectedHTTPstatus(httpStatus int, httpMethod string) bool {
	if httpMethod == "GET" || httpMethod == "LIST" {
		return httpStatus == http.StatusOK
	}

	return httpStatus == http.StatusOK || httpStatus == http.StatusAccepted
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
//
// This is copied directly from the Go source code.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}

	err = tc.SetKeepAlive(true)
	if err != nil {
		return nil, err
	}

	err = tc.SetKeepAlivePeriod(three * time.Minute)
	if err != nil {
		return nil, err
	}

	return tc, nil
}

// serveHTTP sets up an HTTP server to listen for incoming requests.
func serveHTTP(handler HandlerHTTP) {
	server := http.Server{
		ReadHeaderTimeout: ten * time.Second,
		ReadTimeout:       thirty * time.Second,
		IdleTimeout:       five * time.Minute,
		Handler:           &handler,
	}

	listenAddr := fmt.Sprintf("%s:%d", handler.Address, handler.ListenPort)

	listener, err := net.Listen("tcp4", listenAddr)
	if err != nil {
		log.Fatalf("unable create listener, %s", err)
	}

	ln := tcpKeepAliveListener{listener.(*net.TCPListener)}

	log.Infof("Listening for HTTP requests on %d", handler.ListenPort)

	err = server.Serve(ln)
	if err != nil {
		log.Fatalf("unable listen, %s", err)
	}
	handler.C <- "how did I get here!"
}

func JSONresponse(w http.ResponseWriter, jsonResp string) (int, string) {
	w.Header().Set(ContentType, AppJSON)

	_, err := io.WriteString(w, jsonResp)
	if err != nil {
		return http.StatusInternalServerError, fmt.Sprintf("failed to convert response to json, %s", err)
	}

	return http.StatusOK, "OK"
}

// GetReqBody is used to get request body as a string.
func GetReqBody(r *http.Request) (string, error) {
	defer r.Body.Close()

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return "", readingBodyError(err.Error())
	}

	return string(data), nil
}
