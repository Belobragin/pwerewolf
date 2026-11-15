package cloud

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestJoinUrl(t *testing.T) {
	for _, tc := range []struct{ base, path, want string }{
		{"https://x.lambda-url.us-east-2.on.aws/", "/api/v1/predict", "https://x.lambda-url.us-east-2.on.aws/api/v1/predict"},
		{"https://x.lambda-url.us-east-2.on.aws", "api/v1/predict", "https://x.lambda-url.us-east-2.on.aws/api/v1/predict"},
	} {
		if got := joinUrl(tc.base, tc.path); got != tc.want {
			t.Errorf("joinUrl(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}
}

func staticCreds(token string) aws.CredentialsProvider {
	return aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "AKIDTEST", SecretAccessKey: "secret", SessionToken: token}, nil
	})
}

func TestSigV4TransportSigns(t *testing.T) {
	var got *http.Request
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
	}))
	defer srv.Close()

	client := &http.Client{Transport: newSigV4Transport(staticCreds("tok"), "us-east-2", http.DefaultTransport)}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/predict?image_name=a.jpg&folder=f", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	auth := got.Header.Get("Authorization")
	for _, part := range []string{"AWS4-HMAC-SHA256", "Credential=AKIDTEST/", "/us-east-2/lambda/aws4_request"} {
		if !strings.Contains(auth, part) {
			t.Errorf("Authorization %q has no %q", auth, part)
		}
	}
	if got.Header.Get("X-Amz-Date") == "" || got.Header.Get("X-Amz-Security-Token") != "tok" {
		t.Errorf("headers: date %q, token %q", got.Header.Get("X-Amz-Date"), got.Header.Get("X-Amz-Security-Token"))
	}
	if gotBody != "payload" {
		t.Errorf("body = %q, want payload", gotBody)
	}
	// the signer sorts the query parameters
	if q := got.URL.Query(); q.Get("image_name") != "a.jpg" || q.Get("folder") != "f" || len(q) != 2 {
		t.Errorf("query = %q", got.URL.RawQuery)
	}
	if req.Header.Get("Authorization") != "" {
		t.Error("the caller's request was modified")
	}
}

func TestSigV4TransportEmptyBody(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	client := &http.Client{Transport: newSigV4Transport(staticCreds(""), "us-east-2", http.DefaultTransport)}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/predict", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if auth == "" {
		t.Error("request is not signed")
	}
}

func TestSigV4TransportCredentialsError(t *testing.T) {
	creds := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{}, context.DeadlineExceeded
	})
	client := &http.Client{Transport: newSigV4Transport(creds, "us-east-2", http.DefaultTransport)}
	if _, err := client.Get("http://127.0.0.1:1/"); err == nil {
		t.Error("want error when credentials are not available")
	}
}
