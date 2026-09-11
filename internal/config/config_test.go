package config

import "testing"

func TestValidateVisitLogging(t *testing.T) {
	if err := (Config{}).ValidateVisitLogging(); err != nil {
		t.Fatal(err)
	}
	c := Config{VisitLoggingEnabled: true}
	if err := c.ValidateVisitLogging(); err != nil {
		t.Fatal(err)
	}
	c = Config{visitLoggingValue: "wat"}
	if err := c.ValidateVisitLogging(); err == nil {
		t.Fatal("expected invalid flag error")
	}
}

func TestLoadUsesVisitLoggingFlag(t *testing.T) {
	t.Setenv("BUH_DATABASE_URL", "postgres://example")
	t.Setenv("BUH_VISIT_LOGGING_ENABLED", "true")

	c := Load()
	if !c.VisitLoggingEnabled {
		t.Fatal("expected visit logging to be enabled")
	}
}
