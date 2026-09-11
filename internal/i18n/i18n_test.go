package i18n

import "testing"

func TestRateTooltipTranslations(t *testing.T) {
	bundle := NewBundle()
	cases := map[string]string{
		"sr": "Званични курс НБС за USD на дан 04.08.2026",
		"en": "Official NBS exchange rate for USD on 04.08.2026",
		"ru": "Официальный курс НБС для USD на 04.08.2026",
	}
	for lang, want := range cases {
		t.Run(lang, func(t *testing.T) {
			got := bundle.NewLocalizer(lang).TWith(
				"kpo.rate_tooltip",
				"Currency", "USD",
				"Date", "04.08.2026",
			)
			if got != want {
				t.Fatalf("rate tooltip = %q, want %q", got, want)
			}
		})
	}
}
