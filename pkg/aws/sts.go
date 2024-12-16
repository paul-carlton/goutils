package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type STSService struct {
	Client *sts.Client
	config aws.Config
}

func (s *STSService) InitClient() {
	s.Client = sts.NewFromConfig(s.config)
}

func NewSTSService(cfg aws.Config) *STSService {
	s := &STSService{config: cfg}
	s.InitClient()
	return s
}

func (s *STSService) GetAccountID() (*string, error) {
	input := &sts.GetCallerIdentityInput{}
	req, err := s.Client.GetCallerIdentity(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return req.Account, nil
}
