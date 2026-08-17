package blaiserestapi_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	"github.com/jarcoal/httpmock"
)

func TestGetInstrumentSettings(t *testing.T) {
	restAPIURL := "http://localhost"
	serverpark := "foobar"
	instrumentName := "lolcats"
	client := &http.Client{}
	blaiseRestAPI := &blaiserestapi.BlaiseRestAPI{
		BaseURL:    restAPIURL,
		Serverpark: serverpark,
		Client:     client,
	}

	httpmock.ActivateNonDefault(client)
	t.Cleanup(httpmock.DeactivateAndReset)

	url := fmt.Sprintf("%s/api/v2/serverparks/%s/questionnaires/%s/settings", restAPIURL, serverpark, instrumentName)

	t.Run("returns not found error when instrument does not exist", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder("GET", url, httpmock.NewBytesResponder(404, []byte{}))

		instrumentSettings, err := blaiseRestAPI.GetInstrumentSettings(context.Background(), instrumentName)
		if err == nil || err.Error() != "instrument not found" {
			t.Fatalf("error = %v, want instrument not found", err)
		}
		if len(instrumentSettings) != 0 {
			t.Fatalf("instrumentSettings = %+v, want empty", instrumentSettings)
		}
	})

	t.Run("returns instrument settings when instrument exists", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder("GET", url,
			httpmock.NewJsonResponderOrPanic(200, blaiserestapi.InstrumentSettings{{
				Type:           "StrictInterviewing",
				SessionTimeout: 15,
			}}))

		instrumentSettings, err := blaiseRestAPI.GetInstrumentSettings(context.Background(), instrumentName)
		if err != nil {
			t.Fatalf("GetInstrumentSettings() error: %v", err)
		}
		if len(instrumentSettings) != 1 {
			t.Fatalf("len(instrumentSettings) = %d, want 1", len(instrumentSettings))
		}
		if instrumentSettings[0].Type != "StrictInterviewing" {
			t.Fatalf("Type = %q, want StrictInterviewing", instrumentSettings[0].Type)
		}
		if instrumentSettings[0].SessionTimeout != 15 {
			t.Fatalf("SessionTimeout = %d, want 15", instrumentSettings[0].SessionTimeout)
		}
	})

	t.Run("returns error when base URL is invalid", func(t *testing.T) {
		badAPI := &blaiserestapi.BlaiseRestAPI{
			BaseURL:    "://invalid",
			Serverpark: serverpark,
			Client:     client,
		}

		instrumentSettings, err := badAPI.GetInstrumentSettings(context.Background(), instrumentName)
		if err == nil {
			t.Fatal("GetInstrumentSettings() expected error, got nil")
		}
		if err.Error() == "instrument not found" {
			t.Fatalf("error = %q, expected request creation error", err.Error())
		}
		if instrumentSettings != nil {
			t.Fatalf("instrumentSettings = %+v, want nil", instrumentSettings)
		}
	})

	t.Run("returns error when downstream call fails", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder("GET", url,
			httpmock.NewErrorResponder(errors.New("downstream unavailable")))

		instrumentSettings, err := blaiseRestAPI.GetInstrumentSettings(context.Background(), instrumentName)
		if err == nil {
			t.Fatal("GetInstrumentSettings() expected error, got nil")
		}
		if instrumentSettings != nil {
			t.Fatalf("instrumentSettings = %+v, want nil", instrumentSettings)
		}
	})

	t.Run("returns error when response body is invalid JSON", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder("GET", url, httpmock.NewBytesResponder(http.StatusOK, []byte("not-json")))

		instrumentSettings, err := blaiseRestAPI.GetInstrumentSettings(context.Background(), instrumentName)
		if err == nil {
			t.Fatal("GetInstrumentSettings() expected error, got nil")
		}
		if instrumentSettings != nil {
			t.Fatalf("instrumentSettings = %+v, want nil", instrumentSettings)
		}
	})
}

func TestInstrumentSettingsStrictInterviewing(t *testing.T) {
	t.Run("returns first StrictInterviewing block when present", func(t *testing.T) {
		instrumentSettings := blaiserestapi.InstrumentSettings{
			{
				Type:                 "StrictInterviewing",
				SessionTimeout:       15,
				SaveSessionOnTimeout: true,
			},
			{
				Type:           "StrictCati",
				SessionTimeout: 55,
			},
			{
				Type:                 "StrictInterviewing",
				SessionTimeout:       56,
				SaveSessionOnTimeout: false,
			},
		}

		strict := instrumentSettings.StrictInterviewing()
		if strict.SessionTimeout != 15 {
			t.Fatalf("SessionTimeout = %d, want 15", strict.SessionTimeout)
		}
		if strict.Type != "StrictInterviewing" {
			t.Fatalf("Type = %q, want StrictInterviewing", strict.Type)
		}
	})

	t.Run("returns empty settings block when StrictInterviewing is absent", func(t *testing.T) {
		instrumentSettings := blaiserestapi.InstrumentSettings{
			{
				Type:                 "FreeInterviewing",
				SessionTimeout:       15,
				SaveSessionOnTimeout: true,
			},
			{
				Type:           "StrictCati",
				SessionTimeout: 15,
			},
		}

		strict := instrumentSettings.StrictInterviewing()
		if strict != (blaiserestapi.InstrumentSettingsType{}) {
			t.Fatalf("StrictInterviewing() = %+v, want empty", strict)
		}
	})
}
