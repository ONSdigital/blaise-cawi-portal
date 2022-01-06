module github.com/ONSdigital/blaise-cawi-portal

go 1.15

require (
	github.com/blendle/zapdriver v1.3.1
	github.com/dchest/uniuri v0.0.0-20200228104902-7aecb25e1fe5 // indirect
	github.com/gin-contrib/secure v0.0.1
	github.com/gin-contrib/sessions v0.0.4
	github.com/gin-gonic/gin v1.7.7
	github.com/golang-jwt/jwt v3.2.1+incompatible
	github.com/jarcoal/httpmock v1.0.8
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.10.1
	github.com/srbry/gin-csrf v0.0.0-20211221152635-387e490c81de
	github.com/stretchr/objx v0.1.1 // indirect
	github.com/stretchr/testify v1.7.0
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.1
	golang.org/x/net v0.0.0-20211112202133-69e39bad7dc2
	google.golang.org/api v0.50.0
)

replace github.com/gin-contrib/sessions v0.0.4 => github.com/srbry/sessions v0.0.5
