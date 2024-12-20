package ecr

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	semver "github.com/Masterminds/semver/v3"
	awsecr "github.com/aws/aws-sdk-go-v2/service/ecr"
	awsecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/paul-carlton/goutils/pkg/aws"
	"github.com/paul-carlton/goutils/pkg/httpclient"
	"github.com/paul-carlton/goutils/pkg/logging"
	"github.com/paul-carlton/goutils/pkg/miscutils"
)

type index struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Manifests     []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
		Platform  struct {
			Architecture string `json:"architecture"`
			Os           string `json:"os"`
		} `json:"platform"`
		Annotations struct {
			VndDockerReferenceDigest string `json:"vnd.docker.reference.digest"`
			VndDockerReferenceType   string `json:"vnd.docker.reference.type"`
		} `json:"annotations,omitempty"`
	} `json:"manifests"`
}

type manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
	} `json:"config"`
	Layers []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
	} `json:"layers"`
}

type download struct {
	Architecture string `json:"architecture"`
	Config       struct {
		User       string            `json:"User"`
		Env        []string          `json:"Env"`
		Cmd        []string          `json:"Cmd"`
		WorkingDir string            `json:"WorkingDir"`
		Labels     map[string]string `json:"Labels"`
	} `json:"config"`
	Created time.Time `json:"created"`
	History []struct {
		Created    time.Time `json:"created"`
		CreatedBy  string    `json:"created_by"`
		EmptyLayer bool      `json:"empty_layer,omitempty"`
		Comment    string    `json:"comment,omitempty"`
	} `json:"history"`
	Os     string `json:"os"`
	Rootfs struct {
		Type    string   `json:"type"`
		DiffIDs []string `json:"diff_ids"`
	} `json:"rootfs"`
}

type images struct {
	Images
	o           *miscutils.NewObjParams
	awsCfg      aws.Config
	ecrClient   *awsecr.Client
	region      string
	httpReqResp httpclient.ReqResp
}

type Images interface {
	setEcrClient() *awsecr.Client
	gitImageDigest(imageName, imageTag, imageDigest string) (*awsecr.BatchGetImageOutput, error)
	getManifestDigest(imageName, imageTag string) (string, error)
	getConfigDigest(imageName, imageTag, imageDigest string) (string, error)
	describeImages(params *awsecr.DescribeImagesInput) (*awsecr.DescribeImagesOutput, error)
	downloadLayer(downloadURL string) (string, error)
	GetConfigLabels(imageName, imageTag, imageDigest string) (map[string]string, error)

	GetLatestImage(repo, policy string) (string, error)
	GetRunnerVersionLabel(imageName, imageTag string) (string, error)
	ApplyPolicy(policy, tag string) bool
	MaxImage(policy, current, new string) string
}

func NewImages(objParams *miscutils.NewObjParams, awsConfig aws.Config, httpClient *http.Client) Images {
	logging.TraceCall()
	defer logging.TraceExit()

	e := images{
		o:      objParams,
		region: cmp.Or(os.Getenv("AWS_REGION"), "us-west-2"),
		awsCfg: awsConfig,
	}

	var err error
	if e.httpReqResp, err = httpclient.NewReqResp(objParams, nil, httpClient, nil); err != nil {
		e.o.Log.Error("failed to get httpReqResp", "error", err)
	}

	e.ecrClient = e.setEcrClient()

	return &e
}

func (e *images) setEcrClient() *awsecr.Client {
	logging.TraceCall()
	defer logging.TraceExit()

	if e.awsCfg == nil {
		var err error
		e.awsCfg, err = aws.NewAwsConfig(e.o, "", e.region)
		if err != nil {
			e.o.Log.Log(e.o.Ctx, logging.LevelFatal, "failed to get AWS config", "error", err.Error())
		}
	}

	return awsecr.NewFromConfig(e.awsCfg.NewConfig("", e.region))
}

func (e *images) getImageDigest(imageName, imageTag, imageDigest string) (*awsecr.BatchGetImageOutput, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	e.o.Log.Log(e.o.Ctx, slog.LevelDebug, "getting info about image tag", "image", imageName, "tag", imageTag, "digest", imageDigest)

	ids := []awsecrtypes.ImageIdentifier{{
		ImageTag: &imageTag,
	}}

	if len(imageDigest) > 0 {
		ids = []awsecrtypes.ImageIdentifier{{
			ImageDigest: &imageDigest,
		}}
	}

	input := awsecr.BatchGetImageInput{
		RepositoryName: &imageName,
		ImageIds:       ids,
		AcceptedMediaTypes: []string{
			"application/vnd.docker.distribution.manifest.v1+json",
			"application/vnd.docker.distribution.manifest.v2+json",
			"application/vnd.oci.image.manifest.v1+json",
		},
	}
	ctx, cancel := context.WithTimeout(e.o.Ctx, time.Second*60) //nolint: mnd
	defer cancel()
	output, err := e.ecrClient.BatchGetImage(ctx, &input)
	if err != nil {
		return nil, fmt.Errorf("failed to get image digest: %s:%s, error: %w", imageName, imageTag, err)
	}
	return output, nil
}

func (e *images) getManifestDigest(imageName, imageTag string) (string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	output, err := e.getImageDigest(imageName, imageTag, "")
	if err != nil {
		return "", fmt.Errorf("failed to get manifest digest: %s:%s, error: %w", imageName, imageTag, err)
	}

	if logging.LogLevel <= logging.LevelTrace {
		for _, image := range output.Images {
			fmt.Fprintf(e.o.LogOut, "manifest...\n%s\n", *image.ImageManifest)
		}
	}

	if len(output.Images) == 0 {
		return "", nil
	}

	m := &index{}
	if err := json.Unmarshal([]byte(*output.Images[0].ImageManifest), m); err != nil {
		return "", fmt.Errorf("failed to marshal image index: %s:%s, error: %w", imageName, imageTag, err)
	}

	if len(m.Manifests) > 0 {
		e.o.Log.Log(e.o.Ctx, slog.LevelDebug, "image tag", "image", imageName, "tag", imageTag, "manifest digest", m.Manifests[0].Digest)
		return m.Manifests[0].Digest, nil
	}
	return "", nil
}

func (e *images) getConfigDigest(imageName, imageTag, imageDigest string) (string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	output, err := e.getImageDigest(imageName, imageTag, imageDigest)
	if err != nil {
		return "", fmt.Errorf("failed to get config digest: %s:%s, error: %w", imageName, imageTag, err)
	}

	if logging.LogLevel <= logging.LevelTrace {
		for _, image := range output.Images {
			fmt.Fprintf(e.o.LogOut, "manifest...\n%s\n", *image.ImageManifest)
		}
	}

	if len(output.Images) == 0 {
		return "", nil
	}

	m := &manifest{}
	if err := json.Unmarshal([]byte(*output.Images[0].ImageManifest), m); err != nil {
		return "", fmt.Errorf("failed to marshal image index: %s:%s, error: %w", imageName, imageTag, err)
	}

	e.o.Log.Log(e.o.Ctx, slog.LevelDebug, "image tag", "image", imageName, "tag", imageTag, "config digest", m.Config.Digest)
	return m.Config.Digest, nil
}

func (e *images) downloadLayer(downloadURL string) (string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	if err := e.httpReqResp.HTTPreq(&httpclient.Get, &url.URL{Opaque: downloadURL}, nil, nil); err != nil {
		return "", err
	}

	return *e.httpReqResp.RespBody(), nil
}

func (e *images) GetConfigLabels(imageName, imageTag, imageDigest string) (map[string]string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	input := awsecr.GetDownloadUrlForLayerInput{
		RepositoryName: &imageName,
		LayerDigest:    &imageDigest,
	}

	ctx, cancel := context.WithTimeout(e.o.Ctx, time.Second*60) //nolint: mnd
	defer cancel()
	output, err := e.ecrClient.GetDownloadUrlForLayer(ctx, &input)
	if err != nil {
		return nil, fmt.Errorf("failed to get image layers: %s:%s, error: %w", imageName, imageTag, err)
	}

	e.o.Log.Log(e.o.Ctx, slog.LevelDebug, "image layers", "image", imageName, "tag", imageTag)
	if logging.LogLevel <= logging.LevelTrace {
		fmt.Fprintf(e.o.LogOut, "download url...\n%s\n", miscutils.IndentJSON(output, 0, 2)) //nolint: mnd
	}

	data, err := e.downloadLayer(*output.DownloadUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %s:%s, error: %w", imageName, imageTag, err)
	}

	if logging.LogLevel <= logging.LevelTrace {
		fmt.Fprintf(e.o.LogOut, "download data...\n%s\n", data)
	}

	d := &download{}
	if err := json.Unmarshal([]byte(data), d); err != nil {
		return nil, fmt.Errorf("failed to marshal downloaded data: %s:%s, error: %w", imageName, imageTag, err)
	}

	if logging.LogLevel <= logging.LevelTrace {
		fmt.Fprintf(e.o.LogOut, "download loaded...\n%s\n", miscutils.IndentJSON(d, 0, 2)) //nolint: mnd
	}

	if logging.LogLevel <= logging.LevelTrace {
		fmt.Fprintf(e.o.LogOut, "download loaded...\n%s\n", d.Config.Labels)
	}

	return d.Config.Labels, nil
}

func (e *images) GetRunnerVersionLabel(imageName, imageTag string) (string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	digest, err := e.getManifestDigest(imageName, imageTag)
	if err != nil {
		return "", fmt.Errorf("failed to get manifest digest: %s:%s, error: %w", imageName, imageTag, err)
	}

	d, err := e.getConfigDigest(imageName, imageTag, digest)
	if err != nil {
		return "", fmt.Errorf("failed to get config digest: %s:%s, error: %w", imageName, imageTag, err)
	}

	labels, err := e.GetConfigLabels(imageName, imageTag, d)
	if err != nil {
		return "", fmt.Errorf("failed to get image manifest digest: %s:%s, error: %w", imageName, imageTag, err)
	}

	return labels["actions-runner-version"], nil
}

func (e *images) ApplyPolicy(policy, tag string) bool {
	logging.TraceCall()
	defer logging.TraceExit()

	c, err := semver.NewConstraint(policy)
	if err != nil {
		e.o.Log.Log(e.o.Ctx, logging.LevelTrace, "failed to create contraint", "policy", policy)
		return false
	}

	v, err := semver.NewVersion(tag)
	if err != nil {
		e.o.Log.Log(e.o.Ctx, logging.LevelTrace, "failed to create new semver", "tag", tag)
		return false
	}

	a, msgs := c.Validate(v)
	if !a {
		if logging.LogLevel <= logging.LevelTrace {
			fmt.Fprintf(e.o.LogOut, "tag: %s failed validation\n", tag)
			for _, msg := range msgs {
				fmt.Fprintf(e.o.LogOut, "%s\n", msg)
			}
		}
	}
	return a
}

func (e *images) MaxImage(policy, current, new string) string {
	logging.TraceCall()
	defer logging.TraceExit()

	if !e.ApplyPolicy(policy, new) {
		return current
	}

	v, err := semver.NewVersion(new)
	if err != nil {
		e.o.Log.Log(e.o.Ctx, logging.LevelTrace, "failed to create new semver", "tag", new)
		return current
	}

	c, err := semver.NewVersion(current)
	if err != nil {
		e.o.Log.Log(e.o.Ctx, logging.LevelTrace, "failed to create new semver", "tag", current)
		return current
	}

	if v.GreaterThan(c) {
		return v.String()
	}

	return current
}

func (e *images) describeImages(params *awsecr.DescribeImagesInput) (*awsecr.DescribeImagesOutput, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(e.o.Ctx, time.Second*60) //nolint: mnd
	defer cancel()
	output, err := e.ecrClient.DescribeImages(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get images, error: %w", err)
	}
	return output, nil
}

func (e *images) GetLatestImage(repo, policy string) (string, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	latestImage := "0.0.0"

	var oneHundred int32 = 100
	params := awsecr.DescribeImagesInput{
		RepositoryName: &repo,
		Filter: &awsecrtypes.DescribeImagesFilter{
			TagStatus: awsecrtypes.TagStatusTagged,
		},
		MaxResults: &oneHundred,
	}
	for {
		output, err := e.describeImages(&params)
		if err != nil {
			return "", fmt.Errorf("failed to get images, error: %w", err)
		}
		for _, image := range output.ImageDetails {
			for _, i := range image.ImageTags {
				if logging.LogLevel <= logging.LevelTrace {
					fmt.Fprintf(e.o.LogOut, "tag: %s\n", i)
				}
				latestImage = e.MaxImage(policy, latestImage, i)
			}
		}
		if output.NextToken == nil {
			break
		}
		params.NextToken = output.NextToken
	}

	return latestImage, nil
}
