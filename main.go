package main

import (
	"context"
	"io"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/krolaw/zipstream"
)

type PipeWriter struct {
	io.Writer
}

func (w PipeWriter) WriteAt(p []byte, offset int64) (n int, err error) {
	return w.Write(p)
}

type InputEvent struct {
	Records []*Record `json:"records"`
}

type Record struct {
	S3 S3Record `json:"s3"`
}

type S3Record struct {
	Bucket `json:"bucket"`
	Object `json:"object"`
}

type Bucket struct {
	Name string `json:"name"`
}

type Object struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

func unzip(ctx context.Context, downloader *manager.Downloader, uploader *manager.Uploader, bucket, key, outputBucket string) error {
	pipeReader, pipeWriter := io.Pipe()

	wg := sync.WaitGroup{}

	wg.Add(2)

	go func() {
		_, err := downloader.Download(ctx, PipeWriter{pipeWriter}, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			log.Fatal(err)
		}
		wg.Done()
		pipeWriter.Close()
	}()

	go func() {
		data := zipstream.NewReader(pipeReader)
		for {
			header, err := data.Next()
			if err != nil {
				if err != io.EOF {
					log.Fatal(err)
				}
				break
			}

			_, err = uploader.Upload(ctx, &s3.PutObjectInput{
				Bucket: aws.String(outputBucket),
				Key:    aws.String(header.Name),
				Body:   data,
			})
			if err != nil {
				log.Fatal("Failed to upload: ", header.Name)
			}
		}
		wg.Done()
	}()

	wg.Wait()

	return nil
}

func handler(ctx context.Context, event InputEvent) error {
	outputBucket := os.Getenv("OUTPUT_BUCKET")

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	s3client := s3.NewFromConfig(cfg)

	downloader := manager.NewDownloader(s3client)
	uploader := manager.NewUploader(s3client)

	wg := sync.WaitGroup{}

	wg.Add(len(event.Records))

	for _, record := range event.Records {
		bucket := record.S3.Bucket
		object := record.S3.Object
		go unzip(ctx, downloader, uploader, bucket.Name, object.Key, outputBucket)
	}

	wg.Wait()

	return nil
}

func main() {
	lambda.Start(handler)
}
