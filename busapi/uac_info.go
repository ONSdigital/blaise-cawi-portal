package busapi

type UacInfo struct {
	InstrumentName string `json:"instrument_name"`
	CaseID         string `json:"case_id"`
	Disabled       bool   `json:"disabled"`
}

func (uacInfo *UacInfo) InvalidCase() bool {
	return uacInfo.InstrumentName == "" || uacInfo.CaseID == "" ||
		uacInfo.InstrumentName == "unknown" || uacInfo.CaseID == "unknown" || uacInfo.Disabled
}
