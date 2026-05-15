package allino

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Config struct {
	AWSRegion string          `json:"aws_region"`
	Static    *S3StaticConfig `json:"static"`
}

type S3StaticConfig struct {
	BaseEndpoint string `json:"base_endpoint"`
	Key          string `json:"key"`
	Secret       string `json:"secret"`
	Session      string `json:"session"`
	UsePathStyle bool   `json:"use_path_style"`
}

func (c *S3Config) setup(ctx context.Context) (*s3.Client, error) {
	if c.AWSRegion != "" {
		cfg, err := config.LoadDefaultConfig(ctx,
			config.WithRegion("ap-northeast-1"),
		)
		if err != nil {
			return nil, err
		}

		client := s3.NewFromConfig(cfg)
		return client, nil
	}

	if c.Static != nil {
		cfg, err := config.LoadDefaultConfig(ctx,
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(c.Static.Key, c.Static.Secret, c.Static.Session)))
		if err != nil {
			return nil, err
		}

		client := s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(c.Static.BaseEndpoint)
			o.UsePathStyle = c.Static.UsePathStyle
		})
		return client, nil
	}

	return nil, nil
}
