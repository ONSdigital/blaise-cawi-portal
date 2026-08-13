package busapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

//go:generate mockery
type BUSAPIInterface interface {
	GetUACInfo(context.Context, string) (UACInfo, error)
}

type BUSAPI struct {
	BaseURL string
	Client  *http.Client
}

type UACRequest struct {
	UAC string `json:"uac"`
}

func (busApi *BUSAPI) GetUACInfo(ctx context.Context, uac string) (UACInfo, error) {
	response, err := busApi.doGetUACInfo(ctx, uac)
	if err != nil {
		return UACInfo{}, err
	}

	if response.StatusCode == http.StatusNotFound {
		return UACInfo{}, nil
	}

	return busApi.marshalUACResponse(response)
}

func (busApi *BUSAPI) getUACInfoURL() (url string) {
	return fmt.Sprintf("%s/uacs/uac",
		busApi.BaseURL,
	)
}

func (busApi *BUSAPI) doGetUACInfo(ctx context.Context, uac string) (*http.Response, error) {
	uacRequest := UACRequest{UAC: uac}
	uacJSON, err := json.Marshal(uacRequest)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal UAC request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, busApi.getUACInfoURL(),
		bytes.NewReader(uacJSON),
	)

	if err != nil {
		return nil, fmt.Errorf("unable to create UAC info request: %w", err)
	}

	response, err := busApi.Client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("unable to call UAC info endpoint: %w", err)
	}

	return response, nil
}

func (busApi *BUSAPI) marshalUACResponse(response *http.Response) (UACInfo, error) {
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return UACInfo{}, fmt.Errorf("unable to read response body: %w", err)
	}

	var uacInfo UACInfo
	err = json.Unmarshal(body, &uacInfo)
	if err != nil {
		return UACInfo{}, fmt.Errorf("unable to unmarshal UAC response: %w", err)
	}
	return uacInfo, nil
}
