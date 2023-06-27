package blaiserestapi

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

//Generate mocks by running "go generate ./..."
//go:generate mockery --name BlaiseRestApiInterface
type BlaiseRestApiInterface interface {
	GetInstrumentSettings(string) (InstrumentSettings, error)
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

var InstrumentNotFoundError = fmt.Errorf("instrument not found")

func (instrumentSettings InstrumentSettings) StrictInterviewing() InstrumentSettingsType {
	for _, instrumentSettingType := range instrumentSettings {
		if instrumentSettingType.Type == "StrictInterviewing" {
			return instrumentSettingType
		}
	}

	return InstrumentSettingsType{}
}

type BlaiseRestApi struct {
	BaseUrl    string
	Serverpark string
	Client     *http.Client
}

func (blaiseRestApi *BlaiseRestApi) GetInstrumentSettings(instrumentName string) (InstrumentSettings, error) {
	req, err := http.NewRequest("GET", blaiseRestApi.instrumentSettingsUrl(instrumentName), nil)
	if err != nil {
		log.Error("Failed to make new request to blaise rest api")
		return nil, err
	}
	req.Header.Add("Accept", "application/json")
	resp, err := blaiseRestApi.Client.Do(req)
	if err != nil {
		log.Error("Failed to get instrument settings")
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		log.Error(fmt.Sprintf("Questionnaire %s not found", instrumentName))
		return nil, InstrumentNotFoundError
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error(fmt.Sprintf("Error reading response body of %s", resp.Body))
		return nil, err
	}
	var instrumentSettings InstrumentSettings
	err = json.Unmarshal(body, &instrumentSettings)
	if err != nil {
		log.Error(fmt.Sprintf("Could not unmarshall %s", body))
		return nil, err
	}
	return instrumentSettings, nil
}

func (blaiseRestApi *BlaiseRestApi) instrumentSettingsUrl(instrumentName string) string {
	return fmt.Sprintf(
		"%s/api/v2/serverparks/%s/questionnaires/%s/settings",
		blaiseRestApi.BaseUrl,
		blaiseRestApi.Serverpark,
		instrumentName,
	)
}
