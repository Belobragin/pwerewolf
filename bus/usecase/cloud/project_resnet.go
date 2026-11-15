package cloud

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/belobragin/bvv_utils/goutils/awsutil"
	"github.com/belobragin/werewolf/bus/internal/config"
	"go.uber.org/zap"
)

// the crresnet stack exports the function url of the lambda under this key
const lambdaFunctionUrlOutput = "LambdaFunctionUrl"

// ResnetCloudRealization sends the predict requests to the crresnet lambda function url.
type ResnetCloudRealization struct {
	predictUrl string
	transport  http.RoundTripper
}

func (r *ResnetCloudRealization) GetPredictApiUrl() string {
	return r.predictUrl
}

// GetTransport returns the transport that signs the requests: the function url has the AWS_IAM auth type.
func (r *ResnetCloudRealization) GetTransport() http.RoundTripper {
	return r.transport
}

// NewResnetCloudRealization resolves the function url from the outputs of the crresnet cfn stack.
func NewResnetCloudRealization(cf config.CloudConfig, w *zap.Logger) (*ResnetCloudRealization, error) {
	ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
	defer cancel()

	cfg, err := awscfg.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	baseUrl, err := awsutil.GetStackOutput(ctx, cfg, cf.GetResnetStackName(), lambdaFunctionUrlOutput)
	if err != nil {
		return nil, err
	}

	r := &ResnetCloudRealization{
		predictUrl: joinUrl(baseUrl, cf.GetResnetPredictPath()),
		transport:  newSigV4Transport(cfg.Credentials, cfg.Region, http.DefaultTransport),
	}
	w.Info("resnet lambda resolved", zap.String("url", r.predictUrl))
	return r, nil
}

func joinUrl(base, p string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(p, "/")
}
