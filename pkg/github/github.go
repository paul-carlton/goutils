package github

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	githubapi "github.com/google/go-github/v66/github"
	"github.com/paul-carlton/goutils/pkg/logging"
	"github.com/paul-carlton/goutils/pkg/miscutils"
)

const (
	oneHundred = 10
	completed  = "completed"
)

type apiClient struct {
	API
	o            *miscutils.NewObjParams
	dryRun       bool
	gitHubClient *githubapi.Client
	org          string
}

type API interface {
	GetActionsReleases() (map[string]time.Time, error)
	GetRepoVariable(repoName, varName string) (string, error)
	SetRepoVariable(repo, varName, varValue string) error
	GetWorkflowJob(wfName, wfTitle, repo, branch, event string) (int64, string, error)
	SubmitWorkflow(repo, branch, wfName string, inputs map[string]interface{}) error
	GetWorkflowRunByID(repo string, id int64) (*githubapi.WorkflowRun, error)
}

func NewAPIClient(objParams *miscutils.NewObjParams, org, token string, httpClient *http.Client) API {
	logging.TraceCall()
	defer logging.TraceExit()

	g := apiClient{
		o:            objParams,
		dryRun:       strings.EqualFold(os.Getenv("DRY_RUN"), "true"),
		gitHubClient: githubapi.NewClient(httpClient).WithAuthToken(token),
		org:          org,
	}

	return &g
}

func (g *apiClient) GetActionsReleases() (map[string]time.Time, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	releases := []*githubapi.RepositoryRelease{}
	opts := &githubapi.ListOptions{Page: 0, PerPage: oneHundred}
	for {
		release, response, err := g.gitHubClient.Repositories.ListReleases(g.o.Ctx, "actions", "runner", opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get actions runner releases, error: %w", err)
		}

		releases = append(releases, release...)

		if response.NextPage != 0 {
			break
		}
		opts.Page = response.NextPage
	}

	releasesInfo := make(map[string]time.Time)
	for _, rel := range releases {
		releasesInfo[*rel.Name] = rel.PublishedAt.Time
	}

	return releasesInfo, nil
}

func (g *apiClient) GetRepoVariable(repoName, varName string) (string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	variableInfo, _, err := g.gitHubClient.Actions.GetRepoVariable(g.o.Ctx, g.org, repoName, varName)
	if err != nil {
		return "", err
	}
	return variableInfo.Value, nil
}

func (g *apiClient) SetRepoVariable(repo, varName, varValue string) error {
	logging.TraceCall()
	defer logging.TraceExit()

	varInfo := &githubapi.ActionsVariable{
		Name:  varName,
		Value: varValue,
	}
	if g.dryRun {
		g.o.Log.Info("dry run, skipping update of repository variable", "repository", repo, "variable", varName, "value", varValue)
		return nil
	}
	_, err := g.gitHubClient.Actions.UpdateRepoVariable(g.o.Ctx, g.org, repo, varInfo)
	if err != nil {
		return fmt.Errorf("failed to update repo: %s variable: %s, error: %w", repo, varName, err)
	}
	return nil
}

func (g *apiClient) GetWorkflowJob(wfName, wfTitle, repo, branch, event string) (int64, string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	nextPage := 0
	for {
		opts := &githubapi.ListWorkflowRunsOptions{
			Branch: branch,
			Event:  event,
			ListOptions: githubapi.ListOptions{
				PerPage: oneHundred,
				Page:    nextPage,
			},
		}
		runs, response, err := g.gitHubClient.Actions.ListRepositoryWorkflowRuns(g.o.Ctx, g.org, repo, opts)
		if err != nil {
			return 0, "", fmt.Errorf("failed to list workflow runs, error: %w", err)
		}

		var workflowID int64
		if logging.LogLevel <= logging.LevelTrace {
			fmt.Fprintf(g.o.LogOut, "workflows...\n%s\n", miscutils.IndentJSON(runs.WorkflowRuns, 0, 2))
		}
		for _, run := range runs.WorkflowRuns {
			if *(run.Path) == wfName &&
				*(run.DisplayTitle) == wfTitle {
				workflowID = *run.ID
				return workflowID, *run.HTMLURL, nil
			}
		}
		if response.NextPage == 0 {
			break
		}
		nextPage = response.NextPage
	}
	return 0, "", fmt.Errorf("failed to find workflow") //nolint: err113
}

func (g *apiClient) SubmitWorkflow(repo, branch, wfName string, inputs map[string]interface{}) error {
	logging.TraceCall()
	defer logging.TraceExit()

	event := githubapi.CreateWorkflowDispatchEventRequest{
		Ref:    branch,
		Inputs: inputs,
	}

	if logging.LogLevel <= logging.LevelTrace {
		fmt.Fprintf(g.o.LogOut, "input...\n%s\n", miscutils.IndentJSON(event.Inputs, 0, 2))
	}
	response, err := g.gitHubClient.Actions.CreateWorkflowDispatchEventByFileName(g.o.Ctx, g.org, repo, wfName, event)
	if err != nil {
		return fmt.Errorf("failed to trigger workflow: %s, error: %w", wfName, err)
	}
	if response.StatusCode != 204 {
		return fmt.Errorf("workflow dispatch response code: %d", response.StatusCode) //nolint: err113
	}
	return nil
}

func (g *apiClient) GetWorkflowRunByID(repo string, id int64) (*githubapi.WorkflowRun, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	workflow, _, err := g.gitHubClient.Actions.GetWorkflowRunByID(g.o.Ctx, g.org, repo, id)
	if err != nil {
		return nil, err
	}
	if logging.LogLevel <= logging.LevelTrace {
		fmt.Fprintf(g.o.LogOut, "workflow...\n%s\n", miscutils.IndentJSON(workflow, 0, 2))
	}
	return workflow, nil
}
