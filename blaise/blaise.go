package blaise

import "net/url"

type LaunchBlaise struct {
	KeyValue  string `json:"KeyValue"`
	Mode      string `json:"Mode"`
	Language  string `json:"Language,omitempty"`
}

type StartInterview struct {
	RuntimeParameters LaunchBlaise `json:"RuntimeParameters"`
}

func CasePayload(caseID string, welsh bool) LaunchBlaise {
	var language string
	if welsh {
		language = "WLS"
	}
	return LaunchBlaise{
		KeyValue:  caseID,
		Mode:      "CAWI",
		Language:  language,
	}
}

func (blaise LaunchBlaise) Form() url.Values {
	formValues := url.Values{
		"KeyValue":  {blaise.KeyValue},
		"Mode":      {blaise.Mode},
	}
	if blaise.Language != "" {
		formValues["Language"] = []string{blaise.Language}
	}
	return formValues
}
