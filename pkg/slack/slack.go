package slack

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/nabancard/goutils/pkg/httpclient"
	"github.com/nabancard/goutils/pkg/logging"
	"github.com/nabancard/goutils/pkg/miscutils"
)

type messageBody struct {
	Text string `json:"text"`
}

type messages struct {
	Messages
	o           *miscutils.NewObjParams
	dryRun      bool
	postURL     url.URL
	httpReqResp httpclient.ReqResp
}

type Messages interface {
	Post(message string) error
}

func NewMessages(objParams *miscutils.NewObjParams, httpClient *http.Client) Messages {
	logging.TraceCall()
	defer logging.TraceExit()

	s := messages{
		o:      objParams,
		dryRun: strings.EqualFold(os.Getenv("NO_SLACK"), "true"),
		postURL: url.URL{Scheme: "https", Host: "hooks.slack.com",
			Path: fmt.Sprintf("services/%s", os.Getenv("SLACK_CHANNEL_CREDS"))},
	}
	var err error
	if s.httpReqResp, err = httpclient.NewReqResp(objParams, nil, httpClient, nil); err != nil {
		s.o.Log.Error("failed to get httpReqResp", "error", err)
	}

	return &s
}

func (s *messages) Post(message string) error {
	logging.TraceCall()
	defer logging.TraceExit()

	if s.dryRun {
		fmt.Fprint(s.o.LogOut, message)
		return nil
	}

	method := "POST"
	body := messageBody{Text: message}
	if logging.LogLevel <= logging.LevelTrace {
		fmt.Fprintf(s.o.LogOut, "body...\n%s\n", miscutils.IndentJSON(body, 0, 2)) //nolint: mnd
	}
	if err := s.httpReqResp.HTTPreq(&method, &s.postURL, miscutils.IndentJSON(body, 0, 2), nil); err != nil {
		return err
	}

	return nil
}
