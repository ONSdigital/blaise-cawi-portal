package busapi

import (
	"bytes"
	"encoding/json"
	"fmt"
    "io"
	"net/http"
)

//Generate mocks by running "go generate ./..."
//go:generate mockery --name BusApiInterface
type BusApiInterface interface {
	GetUacInfo(string) (UacInfo, error)
}

type BusApi struct {
	BaseUrl string
	Client  *http.Client
}

type UACRequest struct {
	UAC string `json:"uac"`
}

func (busApi *BusApi) GetUacInfo(uac string) (UacInfo, error) {
	response, err := busApi.doGetUacInfo(uac)
	if err != nil {
		return UacInfo{}, err
	}

	if response.StatusCode == http.StatusNotFound {
		return UacInfo{}, nil
	}

	return busApi.marshalUacResponse(response)
}

func (busApi *BusApi) getUACInfoUrl() (url string) {
	return fmt.Sprintf("%s/uacs/uac",
		busApi.BaseUrl,
	)
}

func (busApi *BusApi) doGetUacInfo(uac string) (*http.Response, error) {
	uacRequest := UACRequest{UAC: uac}
	uacJSON, err := json.Marshal(uacRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to Marshal error")
	}

	request, err := http.NewRequest("POST", busApi.getUACInfoUrl(),
		bytes.NewReader(uacJSON),
	)

	if err != nil {
		return nil, err
	}

	return busApi.Client.Do(request)
}

func (busApi *BusApi) marshalUacResponse(response *http.Response) (UacInfo, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return UacInfo{}, fmt.Errorf("unable to read body")
	}
	defer response.Body.Close()

	var uacInfo UacInfo
	err = json.Unmarshal(body, &uacInfo)
	if err != nil {
		return UacInfo{}, fmt.Errorf("unable To Unmarshal Json")
	}
	return uacInfo, nil
}
