package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go-microservice/models"
)

type IntegrationService struct {
	client        *minio.Client
	bucket, runID string
}

func NewIntegrationService(endpoint, key, secret, bucket, runID string, secure bool) (*IntegrationService, error) {
	c, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(key, secret, ""), Secure: secure})
	if err != nil {
		return nil, err
	}
	return &IntegrationService{c, bucket, runID}, nil
}
func (s *IntegrationService) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
	if err != nil {
		if ok, e := s.client.BucketExists(ctx, s.bucket); e == nil && ok {
			return nil
		}
	}
	return err
}
func (s *IntegrationService) Check(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("bucket missing")
	}
	return nil
}
func (s *IntegrationService) Save(ctx context.Context, e models.Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("events/%s/%020d.json", s.runID, e.Sequence)
	_, err = s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/json"})
	return err
}
