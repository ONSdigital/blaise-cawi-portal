package utils_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/ONSdigital/blaise-cawi-portal/utils"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetRequestSource", func() {
	var (
		req     *http.Request
		context *gin.Context
		engine  *gin.Engine
	)

	BeforeEach(func() {
		req, _ = http.NewRequest("GET", "http://localhost:8000", nil)
		req.RemoteAddr = "1.1.1.1"

		context, engine = gin.CreateTestContext(httptest.NewRecorder())
		engine.AppEngine = true
	})

	Context("When the remote address and client IP match", func() {
		It("Return an IP, but not XFF", func() {
			context.Request = req
			requestSource := utils.GetRequestSource(context)
			Expect(requestSource).To(HaveLen(1))
			Expect(requestSource[0].String).To(Equal("1.1.1.1"))
			Expect(requestSource[0].Key).To(Equal("SourceIP"))
		})
	})

	Context("When the remote address and client IP do not match", func() {
		It("Returns an IP and XFF", func() {
			req.Header.Add("X-Appengine-Remote-Addr", "2.2.2.2")
			context.Request = req
			requestSource := utils.GetRequestSource(context)
			Expect(requestSource[0].String).To(Equal("1.1.1.1"))
			Expect(requestSource[0].Key).To(Equal("SourceIP"))
			Expect(requestSource[1].String).To(Equal("2.2.2.2"))
			Expect(requestSource[1].Key).To(Equal("SourceXFF"))
		})
	})
})
