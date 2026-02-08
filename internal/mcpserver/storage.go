package mcpserver

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Storage handles S3 uploads for podcast audio files.
type Storage struct {
	client      *s3.Client
	bucket      string
	cdnBaseURL  string // e.g. "https://podcasts.apresai.dev"
}

// NewStorage creates an S3 storage handler.
func NewStorage(client *s3.Client, bucket, cdnBaseURL string) *Storage {
	return &Storage{client: client, bucket: bucket, cdnBaseURL: cdnBaseURL}
}

// Upload uploads an MP3 file to S3 and returns the S3 key and public URL.
func (s *Storage) Upload(ctx context.Context, podcastID, mp3Path string) (key, url string, err error) {
	key = "audio/" + podcastID + ".mp3"

	f, err := os.Open(mp3Path)
	if err != nil {
		return "", "", fmt.Errorf("open mp3: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", "", fmt.Errorf("stat mp3: %w", err)
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &key,
		Body:          f,
		ContentType:   aws.String("audio/mpeg"),
		ContentLength: aws.Int64(info.Size()),
	})
	if err != nil {
		return "", "", fmt.Errorf("upload to s3: %w", err)
	}

	url = s.cdnBaseURL + "/" + key
	return key, url, nil
}
