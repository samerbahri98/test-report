package traefikmiddlewaresigv4

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Config struct {
	AccessKey    string  `json:"accessKey"`
	SecretKey    string  `json:"secretKey"`
	SessionToken *string `json:"sessionToken,omitempty"`
	Service      string  `json:"service"`
	Endpoint     string  `json:"endpoint"`
	Region       string  `json:"region"`
}

type Plugin struct {
	next   http.Handler
	config *Config
}

const signedHeaders = "host;x-amz-date"

const algorithm = "AWS4-HMAC-SHA256"

func (p *Plugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	method := r.Method
	host := p.config.Endpoint
	uri := r.URL.Path
	query := r.URL.RawQuery

	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	// Read and hash the payload
	var payload []byte
	if r.Body != nil {
		payload, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(payload)) // Reassign to req.Body since ReadAll consumes it
	}
	payloadHash := sha256.Sum256(payload)

	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-date:%s\n",
		host, amzDate)

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method, uri, query, canonicalHeaders, signedHeaders, hex.EncodeToString(payloadHash[:]))
	canonicalRequestHash := sha256.Sum256([]byte(canonicalRequest))
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, p.config.Region, p.config.Service)
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm, amzDate, credentialScope, hex.EncodeToString(canonicalRequestHash[:]))

	// Signature
	signingKey := getSigningKey(p.config.SecretKey, dateStamp, p.config.Region, p.config.Service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	// Authorization header
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, p.config.AccessKey, credentialScope, signedHeaders, signature)

	r.Header.Set("x-amz-date", amzDate)
	r.Header.Set("x-amz-content-sha256", hex.EncodeToString(payloadHash[:]))
	r.Header.Set("Authorization", authorization)

	p.next.ServeHTTP(w, r)
}

func CreateConfig() *Config {
	return &Config{}
}

func New(ctx context.Context, next http.Handler, config *Config) (http.Handler, error) {
	return &Plugin{
		next:   next,
		config: config,
	}, nil
}

// hmacSHA256 returns the HMAC SHA-256 of the data using the provided key.
func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// getSigningKey generates the signing key using the AWS secret key, date, region, and service.
func getSigningKey(secretKey, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}
