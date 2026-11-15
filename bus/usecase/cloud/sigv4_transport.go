package cloud

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const lambdaSigningService = "lambda"

// sigV4Transport signs every request with the credentials of the default aws chain (the role of the instance or task).
type sigV4Transport struct {
	creds   aws.CredentialsProvider
	signer  *v4.Signer
	region  string
	proxied http.RoundTripper
}

func newSigV4Transport(creds aws.CredentialsProvider, region string, proxied http.RoundTripper) *sigV4Transport {
	return &sigV4Transport{creds: creds, signer: v4.NewSigner(), region: region, proxied: proxied}
}

func (t *sigV4Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil && req.Body != http.NoBody {
		b, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
		body = b
	}

	creds, err := t.creds.Retrieve(req.Context())
	if err != nil {
		return nil, fmt.Errorf("retrieve aws credentials: %w", err)
	}

	// a transport must not modify the caller's request
	signed := req.Clone(req.Context())
	if len(body) > 0 {
		signed.Body = io.NopCloser(bytes.NewReader(body))
	}
	sum := sha256.Sum256(body)
	if err = t.signer.SignHTTP(req.Context(), creds, signed, hex.EncodeToString(sum[:]),
		lambdaSigningService, t.region, time.Now()); err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}
	return t.proxied.RoundTrip(signed)
}
