package app

import (
	"net/http"
	"strings"

	m "github.com/belobragin/bvv_utils/goutils/middleware"
	metrica "github.com/belobragin/bvv_utils/goutils/usecase"
	_ "github.com/belobragin/werewolf/bus/docs/apidoc"
	"github.com/swaggo/swag/v2"
)

func (l *ApiClientStruct) GetDebug() bool {
	return l.ApiClientI.GetDebug()
}

const swaggerPageTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Werewolf API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    const spec = __SWAGGER_SPEC__;
    SwaggerUIBundle({
      spec,
      dom_id: '#swagger-ui',
      deepLinking: true,
      defaultModelsExpandDepth: 1,
      displayRequestDuration: true
    });
  </script>
</body>
</html>`

func (l *ApiClientStruct) SwaggerDocsHandler(w http.ResponseWriter, r *http.Request) {
	var (
		h = &metrica.ServiceApiCallMetricaLabels{
			Model:  "werewolf_docs_swagger",
			Method: r.Method,
			Code:   http.StatusOK,
		}
	)
	zapstruct := l.GetLog()
	zapstruct.Info("writing swagger info")

	docJSON, err := swag.ReadDoc("swagger")
	if err != nil {
		h.Code = http.StatusInternalServerError
		if metricErr := l.GetApiMetric().SetStatusCodeApiCallMetrica(h); metricErr != nil {
			zapstruct.Error("Error on metrica processing")
		}
		m.StandardRespond(zapstruct, w, r, h.Code, "failed to load swagger docs")
		return
	}

	if metricErr := l.GetApiMetric().SetStatusCodeApiCallMetrica(h); metricErr != nil {
		zapstruct.Error("Error on metrica processing")
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := strings.Replace(swaggerPageTemplate, "__SWAGGER_SPEC__", docJSON, 1)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(page))
}
