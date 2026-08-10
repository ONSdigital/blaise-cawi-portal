package busapi_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/busapi"
	"github.com/jarcoal/httpmock"
)

func TestBusApiGetUacInfo(t *testing.T) {
	baseURL := "http://localhost"
	client := &http.Client{}
	api := &busapi.BUSAPI{BaseURL: baseURL, Client: client}
	uac := "123456789012"

	httpmock.ActivateNonDefault(client)
	t.Cleanup(httpmock.DeactivateAndReset)

	t.Run("returns UAC info for a valid UAC", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder("POST", fmt.Sprintf("%s/uacs/uac", baseURL),
			httpmock.NewJsonResponderOrPanic(200, busapi.UACInfo{InstrumentName: "foo", CaseID: "bar"}))

		uacInfo, err := api.GetUACInfo(context.Background(), uac)
		if err != nil {
			t.Fatalf("GetUACInfo() unexpected error: %v", err)
		}
		if uacInfo.InstrumentName != "foo" {
			t.Fatalf("InstrumentName = %q, want %q", uacInfo.InstrumentName, "foo")
		}
		if uacInfo.CaseID != "bar" {
			t.Fatalf("CaseID = %q, want %q", uacInfo.CaseID, "bar")
		}
	})

	t.Run("returns an error and empty UAC info on bad response", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder("POST", fmt.Sprintf("%s/uacs/uac", baseURL),
			httpmock.NewJsonResponderOrPanic(500, "nil"))

		uacInfo, err := api.GetUACInfo(context.Background(), uac)
		if err == nil {
			t.Fatal("GetUACInfo() expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "unable to unmarshal UAC response") {
			t.Fatalf("error %q does not contain expected substring", err.Error())
		}
		if uacInfo.InstrumentName != "" || uacInfo.CaseID != "" {
			t.Fatalf("UACInfo = %+v, want empty fields", uacInfo)
		}
	})
}
