package webserver_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/ONSdigital/blaise-cawi-portal/webserver"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/onsi/ginkgo/extensions/table"
)

var _ = Describe("Security Controller", func() {
	var (
		httpRouter         *gin.Engine
		securityController = &webserver.SecurityController{}
	)

	BeforeEach(func() {
		httpRouter = gin.Default()
		securityController.AddRoutes(httpRouter)
	})

	DescribeTable("TRACE",
		func(path string) {

			httpRecorder := httptest.NewRecorder()
			req, _ := http.NewRequest("TRACE", path, nil)
			httpRouter.ServeHTTP(httpRecorder, req)

			Expect(httpRecorder.Code).To(Equal(http.StatusMethodNotAllowed))
		},
		Entry("root", "/"),
		Entry("short path", "/foo"),
		Entry("longer path", "/foo/bar"),
		Entry("long path", "/foo/bar/baz"),
	)

	DescribeTable("TRACK",
		func(path string) {

			httpRecorder := httptest.NewRecorder()
			req, _ := http.NewRequest("TRACK", path, nil)
			httpRouter.ServeHTTP(httpRecorder, req)

			Expect(httpRecorder.Code).To(Equal(http.StatusMethodNotAllowed))
		},
		Entry("root", "/"),
		Entry("short path", "/foo"),
		Entry("longer path", "/foo/bar"),
		Entry("long path", "/foo/bar/baz"),
	)
})
