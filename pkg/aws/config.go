package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go/middleware"

	"github.com/nabancard/goutils/pkg/logging"
	"github.com/nabancard/goutils/pkg/miscutils"
)

type configs map[string]aws.Config

type cfg struct {
	Config
	o              *miscutils.NewObjParams
	middlewareFunc string
	configs        configs
}

type Config interface {
	NewConfig(profile, region string) aws.Config
}

func NewAwsConfig(newObjParams *miscutils.NewObjParams, profile, region string) (Config, error) {
	logging.TraceCall()
	defer logging.TraceExit()

	c := cfg{
		o:              newObjParams,
		middlewareFunc: defaultFunc,
		configs:        make(configs),
	}

	c.configs[getProfileRegionName(profile, region)] = c.NewConfig(profile, region)
	return &c, nil
}

func middlewareImpl(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
	logging.TraceCall()
	defer logging.TraceExit()
	return next.HandleInitialize(ctx, in)
}

const (
	defaultFunc = "default"
)

var (
	middlewareFuncs = map[string]MiddleWareInitFunc{ //nolint: gochecknoglobals
		"default": middlewareImpl,
	}
)

type MiddleWareInitFunc func(context.Context, middleware.InitializeInput, middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error)

func (c *cfg) NewConfig(profile, region string) aws.Config {
	logging.TraceCall()
	defer logging.TraceExit()

	if cfg, ok := c.configs[getProfileRegionName(profile, region)]; ok {
		return cfg
	}

	cfg, err := config.LoadDefaultConfig(c.o.Ctx, config.WithRegion(region),
		config.WithSharedConfigProfile(profile))
	if err != nil {
		c.o.Log.Error("failed to load AWS SDK config", "error", err.Error())
	}

	cfg.APIOptions = append(cfg.APIOptions, func(stack *middleware.Stack) error {
		// Attach the custom middleware to the beginning of the Initialize step
		return stack.Initialize.Add(middleware.InitializeMiddlewareFunc(c.middlewareFunc, middlewareFuncs[c.middlewareFunc]), middleware.Before)
	})

	c.configs[getProfileRegionName(profile, region)] = cfg
	return cfg
}

func getProfileRegionName(profile, region string) string {
	profileName := "default"
	if len(profile) > 0 {
		profileName = profile
	}
	return fmt.Sprintf("%s-%s", profileName, region)
}
