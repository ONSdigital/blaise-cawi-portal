package busapi

import "time"

const (
	MAX_POSTCODE_ATTEMPTS     = 5
	POSTCODE_ATTEMPT_TIMEOUT  = time.Duration(30 * time.Minute)
	POSTCODE_TIMESTAMP_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"
)

type UacInfo struct {
	InstrumentName           string `json:"instrument_name"`
	CaseID                   string `json:"case_id"`
	PostcodeAttempts         int    `json:"postcode_attempts"`
	PostcodeAttemptTimestamp string `json:"postcode_attempt_timestamp"`
}

func (uacInfo UacInfo) ParsePostcodeAttemptTimestamp() (time.Time, error) {
	return time.Parse(POSTCODE_TIMESTAMP_FORMAT, uacInfo.PostcodeAttemptTimestamp)
}

func (uacInfo UacInfo) PostcodeAttemptsExpired() (bool, error) {
	postcodeAttemptTimestamp, err := uacInfo.ParsePostcodeAttemptTimestamp()
	if err != nil {
		return false, err
	}
	if time.Now().UTC().Before(postcodeAttemptTimestamp.Add(POSTCODE_ATTEMPT_TIMEOUT)) {
		return false, nil
	}
	return true, nil
}

func (uacInfo UacInfo) TooManyUnexpiredAttempts() (bool, error) {
	if uacInfo.PostcodeAttempts < MAX_POSTCODE_ATTEMPTS {
		return false, nil
	}
	expired, err := uacInfo.PostcodeAttemptsExpired()
	return !expired, err
}

func (uacInfo UacInfo) TooManyAttempts() bool {
	return uacInfo.PostcodeAttempts >= MAX_POSTCODE_ATTEMPTS
}
