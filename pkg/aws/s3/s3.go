package aws

import (
	"context"
	"os"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/paul-carlton/goutils/pkg/aws"
	"github.com/paul-carlton/goutils/pkg/logging"
	"github.com/paul-carlton/goutils/pkg/miscutils"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
)

type s3Service struct {
	S3service
	o              *miscutils.NewObjParams
	awsCfg         aws.Config
	s3Client       *s3.Client
	profile        string
	region         string
	middlewareFunc string
	downloader     *manager.Downloader
	uploader       *manager.Uploader
}

type S3service interface {
	InitClient() *s3.Client
	ListS3Buckets() (*s3.ListBucketsOutput, error)
	ListObjectsFromS3(params *s3.ListObjectsV2Input) ([]types.Object, error)
	DownloadFileFromS3(bucket, key, outfile string) error
	UploadDataToS3(bucket, key, data string) error
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

func (s *s3Service) InitClient() *s3.Client {
	if s.awsCfg == nil {
		var err error
		s.awsCfg, err = aws.NewAwsConfig(s.o, s.profile, s.region)
		if err != nil {
			s.o.Log.Log(s.o.Ctx, logging.LevelFatal, "failed to get AWS config", "error", err.Error())
		}
	}
	s.s3Client = s3.NewFromConfig(s.awsCfg.NewConfig(s.profile, s.region))
	return s.s3Client
}

func NewS3service(objParams *miscutils.NewObjParams, awsConfig aws.Config, profile, region string) S3service {
	logging.TraceCall()
	defer logging.TraceExit()

	s := s3Service{
		o:              objParams,
		region:         region,
		profile:        profile,
		middlewareFunc: defaultFunc,
		awsCfg:         awsConfig,
	}

	s.InitClient()
	s.downloader = manager.NewDownloader(s.s3Client)
	s.uploader = manager.NewUploader(s.s3Client)
	return &s
}

func (s *s3Service) UploadDataToS3(bucket, key, data string) error {
	ctx, cancel := context.WithTimeout(s.o.Ctx, time.Second*60) //nolint: mnd
	defer cancel()
	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: awssdk.String(bucket),
		Key:    awssdk.String(key),
		Body:   strings.NewReader(data),
	})
	return err
}

func (s *s3Service) DownloadFileFromS3(bucket, key, outfile string) error {
	file, err := os.Create(outfile)
	miscutils.CheckError(err)
	defer file.Close()
	ctx, cancel := context.WithTimeout(s.o.Ctx, time.Second*60) //nolint: mnd
	defer cancel()
	_, err = s.downloader.Download(ctx, file, &s3.GetObjectInput{
		Bucket: awssdk.String(bucket),
		Key:    awssdk.String(key),
	})
	return err
}

func (s *s3Service) ListS3Buckets() (*s3.ListBucketsOutput, error) {
	ctx, cancel := context.WithTimeout(s.o.Ctx, time.Second*60) //nolint: mnd
	defer cancel()
	return s.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
}

func (s *s3Service) ListObjectsFromS3(params *s3.ListObjectsV2Input) ([]types.Object, error) {
	ctx, cancel := context.WithTimeout(s.o.Ctx, time.Second*60) //nolint: mnd
	defer cancel()
	objs, err := s.s3Client.ListObjectsV2(ctx, params)
	return objs.Contents, err
}
