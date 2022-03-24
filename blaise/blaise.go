package blaise

import "net/url"

type LaunchBlaise struct {
	KeyValue  string `json:"KeyValue"`
	Mode      string `json:"Mode"`
	LayoutSet string `json:"LayoutSet"`
	Language  string `json:"Language,omitempty"`
}

type StartInterview struct {
	RuntimeParameters LaunchBlaise `json:"RuntimeParameters"`
}

type ExecuteAction struct {
	Actions []Action `json:"Actions"`
}

type Action struct {
	Key int `json:"Key"`
	Value string `json:"Value"`
}

func CasePayload(caseID string, welsh bool) LaunchBlaise {
	var language string
	if welsh {
		language = "WLS"
	}
	return LaunchBlaise{
		KeyValue:  caseID,
		Mode:      "CAWI",
		LayoutSet: "CAWI-Web_Large",
		Language:  language,
	}
}

func (blaise LaunchBlaise) Form() url.Values {
	formValues := url.Values{
		"KeyValue":  {blaise.KeyValue},
		"Mode":      {blaise.Mode},
		"LayoutSet": {blaise.LayoutSet},
	}
	if blaise.Language != "" {
		formValues["Language"] = []string{blaise.Language}
	}
	return formValues
}
