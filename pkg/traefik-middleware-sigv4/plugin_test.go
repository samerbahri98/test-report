package traefikmiddlewaresigv4_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	plugin "github.com/samerbahri98/test-report/pkg/traefik-middleware-sigv4"

	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsc "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func prepareObject(bucketName, objectName *string, c *plugin.Config) error {
	sdkConfig, err := awsc.LoadDefaultConfig(context.TODO(), awsc.WithRegion(c.Region), awsc.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, "")))

	if err != nil {
		return err
	}

	s3Client := s3.NewFromConfig(sdkConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://play.min.io")
		o.UsePathStyle = true
	})

	output, err := s3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})

	if err != nil {
		return err
	}

	bucketExists := false

	for _, bucket := range output.Buckets {
		bucketExists = aws.ToString(bucket.Name) == aws.ToString(bucketName)
		if bucketExists {
			break
		}
	}

	if !bucketExists {
		if _, err := s3Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
			Bucket: bucketName,
		}); err != nil {
			return err
		}
	}

	objectContent := "<h1>hi</h1>"
	objectReader := strings.NewReader(objectContent)

	if _, err := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: bucketName,
		Key:    objectName,
		Body:   objectReader,
	}); err != nil {
		return err
	}

	return nil
}

func TestHandler(t *testing.T) {
	c := plugin.CreateConfig()
	c.AccessKey = "Q3AM3UQ867SPQQA43P2F"
	c.SecretKey = "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG"
	c.Service = "s3"
	c.Endpoint = "play.min.io"
	c.Region = "us-east-1"

	ctx := context.Background()

	// Make Bucket
	bucketName := aws.String("treafikmiddlewares3v4sig")
	objectName := aws.String("index.html")

	if err := prepareObject(bucketName, objectName, c); err != nil {
		t.Fatal(err)
	}

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	handler, err := plugin.New(ctx, next, c)

	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	reqUrl := fmt.Sprintf("http://%s/%s/%s", c.Endpoint, *bucketName, *objectName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	if err != nil {
		t.Fatal(err)
	}

	handler.ServeHTTP(recorder, req)

	res := recorder.Result()
	defer res.Body.Close()

	body, err := io.ReadAll(recorder.Result().Body)

	if err != nil {
		t.Fatal(err)
	}

	if recorder.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected status OK; got %v: %v", recorder.Result().StatusCode, string(body))
	}

}
