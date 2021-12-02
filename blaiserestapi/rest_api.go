package blaiserestapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
		return nil, err
	}
	req.Header.Add("Accept", "application/json")
	resp, err := blaiseRestApi.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("instrument not found")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var instrumentSettings InstrumentSettings
	err = json.Unmarshal(body, &instrumentSettings)
	if err != nil {
		return nil, err
	}
	return instrumentSettings, nil
}

func (blaiseRestApi *BlaiseRestApi) instrumentSettingsUrl(instrumentName string) string {
	return fmt.Sprintf(
		"%s/api/v1/serverparks/%s/instruments/%s/settings",
		blaiseRestApi.BaseUrl,
		blaiseRestApi.Serverpark,
		instrumentName,
	)
}
