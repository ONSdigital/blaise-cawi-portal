module github.com/ONSdigital/blaise-cawi-portal

go 1.15

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.0.0
	github.com/blendle/zapdriver v1.3.1
	github.com/gin-contrib/secure v0.0.1
	github.com/gin-contrib/sessions v0.0.3
	github.com/gin-gonic/gin v1.7.4
	github.com/golang-jwt/jwt v3.2.1+incompatible
	github.com/gorilla/sessions v1.2.1 // indirect
	github.com/jarcoal/httpmock v1.0.8
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.10.1
	github.com/stretchr/testify v1.7.0
	github.com/utrack/gin-csrf v0.0.0-20190424104817-40fb8d2c8fca
	go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.25.0
	go.opentelemetry.io/otel/sdk v1.0.1
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.1
	google.golang.org/api v0.51.0
)
