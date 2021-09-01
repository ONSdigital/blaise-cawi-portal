package blaiserestapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

//Generate mocks by running "go generate ./..."
//go:generate mockery --name BlaiseRestApiInterface
type BlaiseRestApiInterface interface {
	GetPostCode(string, string) (string, error)
}

type BlaiseRestApi struct {
	BaseUrl    string
	Serverpark string
	Client     *http.Client
}

func (blaiseRestApi *BlaiseRestApi) GetPostCode(instrumentName, caseID string) (string, error) {
	req, err := http.NewRequest("GET", blaiseRestApi.postCodeUrl(instrumentName, caseID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Accept", "application/json")
	resp, err := blaiseRestApi.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("case not found")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var postcode string
	err = json.Unmarshal(body, &postcode)
	if err != nil {
		log.Printf("Postcode request url: '%s'", blaiseRestApi.postCodeUrl(instrumentName, caseID))
		log.Printf("Postcode response code: '%v'", resp.StatusCode)
		log.Printf("Get postcode body: '%s'", string(body))
	}
	return postcode, err
}

func (blaiseRestApi *BlaiseRestApi) postCodeUrl(instrumentName, caseID string) string {
	return fmt.Sprintf(
		"%s/api/v1/serverparks/%s/instruments/%s/cases/%s/postcode",
		blaiseRestApi.BaseUrl,
		blaiseRestApi.Serverpark,
		instrumentName,
		caseID,
	)
}
