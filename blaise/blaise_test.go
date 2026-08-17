package blaise

import "testing"

func TestCasePayloadForEnglish(t *testing.T) {
	payload := CasePayload("12345", false)

	if payload.KeyValue != "12345" {
		t.Fatalf("expected KeyValue 12345, got %q", payload.KeyValue)
	}
	if payload.Mode != "CAWI" {
		t.Fatalf("expected Mode CAWI, got %q", payload.Mode)
	}
	if payload.Language != "" {
		t.Fatalf("expected empty Language for english case, got %q", payload.Language)
	}
}

func TestCasePayloadForWelsh(t *testing.T) {
	payload := CasePayload("12345", true)

	if payload.Language != "WLS" {
		t.Fatalf("expected Language WLS for welsh case, got %q", payload.Language)
	}
}

func TestLaunchBlaiseFormWithoutLanguage(t *testing.T) {
	form := LaunchBlaise{KeyValue: "abc", Mode: "CAWI"}.Form()

	if form.Get("KeyValue") != "abc" {
		t.Fatalf("expected KeyValue abc, got %q", form.Get("KeyValue"))
	}
	if form.Get("Mode") != "CAWI" {
		t.Fatalf("expected Mode CAWI, got %q", form.Get("Mode"))
	}
	if form.Get("Language") != "" {
		t.Fatalf("expected Language to be omitted, got %q", form.Get("Language"))
	}
}

func TestLaunchBlaiseFormWithLanguage(t *testing.T) {
	form := LaunchBlaise{KeyValue: "abc", Mode: "CAWI", Language: "WLS"}.Form()

	if form.Get("Language") != "WLS" {
		t.Fatalf("expected Language WLS, got %q", form.Get("Language"))
	}
}
