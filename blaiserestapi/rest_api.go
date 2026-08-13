package blaiserestapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"
)

//go:generate mockery
type BlaiseRestAPIInterface interface {
	GetInstrumentSettings(context.Context, string) (InstrumentSettings, error)
}

type InstrumentSettingsType struct {
	Type                   string `json:"type"`
	SaveSessionOnTimeout   bool   `json:"saveSessionOnTimeout"`
	SaveSessionOnQuit      bool   `json:"saveSessionOnQuit"`
	DeleteSessionOnTimeout bool   `json:"deleteSessionOnTimeout"`
	DeleteSessionOnQuit    bool   `json:"deleteSessionOnQuit"`
	SessionTimeout         int    `json:"sessionTimeout"`
	ApplyRecordLocking     bool   `json:"applyRecordLocking"`
}

type InstrumentSettings []InstrumentSettingsType

var InstrumentNotFoundError = errors.New("instrument not found")

func (instrumentSettings InstrumentSettings) StrictInterviewing() InstrumentSettingsType {
	for _, instrumentSettingType := range instrumentSettings {
		if instrumentSettingType.Type == "StrictInterviewing" {
			return instrumentSettingType
		}
	}

	return InstrumentSettingsType{}
}

type BlaiseRestAPI struct {
	BaseURL    string
	Serverpark string
	Client     *http.Client
	Logger     *zap.Logger
}

func (blaiseRestApi *BlaiseRestAPI) logger() *zap.Logger {
	if blaiseRestApi.Logger != nil {
		return blaiseRestApi.Logger
	}
	return zap.L()
}

func (blaiseRestApi *BlaiseRestAPI) GetInstrumentSettings(ctx context.Context, instrumentName string) (InstrumentSettings, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blaiseRestApi.instrumentSettingsURL(instrumentName), nil)
	if err != nil {
		blaiseRestApi.logger().Error("Failed to create request to Blaise REST API",
			zap.String("instrumentName", instrumentName),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create Blaise REST API request: %w", err)
	}
	req.Header.Add("Accept", "application/json")
	resp, err := blaiseRestApi.Client.Do(req)
	if err != nil {
		blaiseRestApi.logger().Error("Failed to call Blaise REST API instrument settings endpoint",
			zap.String("instrumentName", instrumentName),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to call Blaise REST API instrument settings endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		blaiseRestApi.logger().Warn("Questionnaire not found in Blaise REST API",
			zap.String("instrumentName", instrumentName),
		)
		return nil, InstrumentNotFoundError
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		blaiseRestApi.logger().Error("Failed to read instrument settings response body",
			zap.String("instrumentName", instrumentName),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to read instrument settings response body: %w", err)
	}
	var instrumentSettings InstrumentSettings
	err = json.Unmarshal(body, &instrumentSettings)
	if err != nil {
		blaiseRestApi.logger().Error("Failed to unmarshal instrument settings response",
			zap.String("instrumentName", instrumentName),
			zap.Int("responseBytes", len(body)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to unmarshal instrument settings response: %w", err)
	}
	return instrumentSettings, nil
}

func (blaiseRestApi *BlaiseRestAPI) instrumentSettingsURL(instrumentName string) string {
	return fmt.Sprintf(
		"%s/api/v2/serverparks/%s/questionnaires/%s/settings",
		blaiseRestApi.BaseURL,
		blaiseRestApi.Serverpark,
		instrumentName,
	)
}
