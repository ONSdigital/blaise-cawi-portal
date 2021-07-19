package blaise

import "net/url"

type LaunchBlaise struct {
	KeyValue  string `json:"KeyValue"`
	Mode      string `json:"Mode"`
	LayoutSet string `json:"LayoutSet"`
}

type StartInterview struct {
	RuntimeParameters LaunchBlaise `json:"RuntimeParameters"`
}

func CasePayload(caseID string) LaunchBlaise {
	return LaunchBlaise{
		KeyValue:  caseID,
		Mode:      "CAWI",
		LayoutSet: "CAWI-Web_Large",
	}
}

func (blaise LaunchBlaise) Form() url.Values {
	return url.Values{
		"KeyValue":  {blaise.KeyValue},
		"Mode":      {blaise.Mode},
		"LayoutSet": {blaise.LayoutSet},
	}
}
