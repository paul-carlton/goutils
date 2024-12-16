package eks

import (
	"cmp"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	awseks "github.com/aws/aws-sdk-go-v2/service/eks"
	awsekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/smithy-go/middleware"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"

	"github.com/nabancard/goutils/pkg/aws"
	"github.com/nabancard/goutils/pkg/logging"
	"github.com/nabancard/goutils/pkg/miscutils"
)

type clusters struct {
	Clusters
	o              *miscutils.NewObjParams
	awsCfg         aws.Config
	eksClient      *awseks.Client
	region         string
	middlewareFunc string
}

func middlewareImpl(ctx context.Context, //nolint: unused
	in middleware.InitializeInput,
	next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
	logging.TraceCall()
	defer logging.TraceExit()
	return next.HandleInitialize(ctx, in)
}

const (
	defaultFunc = "default"
)

var (
	middlewareFuncs = map[string]MiddleWareInitFunc{ //nolint: gochecknoglobals,unused
		"default": middlewareImpl,
	}
)

type MiddleWareInitFunc func(context.Context, middleware.InitializeInput, middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error)

type Clusters interface {
	setEksClient() *awseks.Client
	describeCluster(in *awseks.DescribeClusterInput) (*awseks.DescribeClusterOutput, error)
	matchTags(clusterTags, tags map[string]string) bool

	GetK8sConfig(cluster *awsekstypes.Cluster) (*rest.Config, error)
	GetClustersByTags(tags map[string]string) ([]*awsekstypes.Cluster, error)
}

func NewClusters(objParams *miscutils.NewObjParams, awsConfig aws.Config) Clusters {
	logging.TraceCall()
	defer logging.TraceExit()

	e := clusters{
		o:              objParams,
		region:         cmp.Or(os.Getenv("AWS_REGION"), "us-west-2"),
		middlewareFunc: defaultFunc,
		awsCfg:         awsConfig,
	}

	e.eksClient = e.setEksClient()

	return &e
}

func (e *clusters) setEksClient() *awseks.Client {
	logging.TraceCall()
	defer logging.TraceExit()

	if e.awsCfg == nil {
		var err error
		e.awsCfg, err = aws.NewAwsConfig(e.o, "", e.region)
		if err != nil {
			e.o.Log.Log(e.o.Ctx, logging.LevelFatal, "failed to get AWS config", "error", err.Error())
		}
	}
	return awseks.NewFromConfig(e.awsCfg.NewConfig("", e.region))
}

func (e *clusters) GetK8sConfig(cluster *awsekstypes.Cluster) (*rest.Config, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	gen, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, err
	}
	opts := &token.GetTokenOptions{
		ClusterID: *cluster.Name,
	}
	tok, err := gen.GetWithOptions(opts)
	if err != nil {
		return nil, err
	}
	ca, err := base64.StdEncoding.DecodeString(*cluster.CertificateAuthority.Data)
	if err != nil {
		return nil, err
	}
	config := &rest.Config{
		Host:        *cluster.Endpoint,
		BearerToken: tok.Token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: ca,
		},
	}
	return config, nil
}

func (e *clusters) matchTags(clusterTags, tags map[string]string) bool {
	logging.TraceCall()
	defer logging.TraceExit()

	for tagName, tagValue := range tags {
		if value, ok := clusterTags[tagName]; ok {
			if value != tagValue {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

func (e *clusters) describeCluster(in *awseks.DescribeClusterInput) (*awseks.DescribeClusterOutput, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	ctx, cancel := context.WithTimeout(e.o.Ctx, time.Second*60) //nolint: mnd
	defer cancel()
	return e.eksClient.DescribeCluster(ctx, in)
}

func (e *clusters) GetClustersByTags(tags map[string]string) ([]*awsekstypes.Cluster, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	var oneHundred int32 = 100
	input := &awseks.ListClustersInput{
		MaxResults: &oneHundred,
	}

	ctx, cancel := context.WithTimeout(e.o.Ctx, time.Second*60) //nolint: mnd
	defer cancel()
	output, err := e.eksClient.ListClusters(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters, error: %w", err)
	}

	var matchingClusters = make([]*awsekstypes.Cluster, 0, 10) //nolint: mnd
	for _, cluster := range output.Clusters {
		in := &awseks.DescribeClusterInput{
			Name: &cluster,
		}
		clusterInfo, err := e.describeCluster(in)
		if err != nil {
			return nil, fmt.Errorf("failed to describe cluster: %s, error: %w", cluster, err)
		}
		if logging.LogLevel <= logging.LevelTrace {
			fmt.Fprintf(e.o.LogOut, "cluster info...\n%s\n", miscutils.IndentJSON(clusterInfo, 0, 2)) //nolint: mnd
		}

		if e.matchTags(clusterInfo.Cluster.Tags, tags) {
			matchingClusters = append(matchingClusters, clusterInfo.Cluster)
		}
	}
	return matchingClusters, nil
}
