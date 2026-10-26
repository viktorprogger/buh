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

func TestKPOOriginalAmountTranslations(t *testing.T) {
	bundle := NewBundle()
	cases := map[string]struct {
		product string
		service string
		total   string
		note    string
		tooltip string
	}{
		"sr": {
			product: "Продаја роба, RSD",
			service: "Услуге, RSD",
			total:   "Укупно, RSD",
			note:    "Износи у табели су у RSD; оригинални износ у страној валути приказан је испод.",
			tooltip: "Оригинални износ пре прерачунавања у RSD. У PDF-у КПО је само износ у RSD.",
		},
		"en": {
			product: "Goods sales, RSD",
			service: "Services, RSD",
			total:   "Total, RSD",
			note:    "Amounts in the table are in RSD; the original foreign-currency amount is shown below.",
			tooltip: "Original amount before conversion to RSD. The KPO PDF contains only the RSD amount.",
		},
		"ru": {
			product: "Продажа товаров, RSD",
			service: "Услуги, RSD",
			total:   "Итого, RSD",
			note:    "Суммы в таблице указаны в RSD; исходная сумма в иностранной валюте показана ниже.",
			tooltip: "Исходная сумма до пересчёта в RSD. В PDF КПО указывается только сумма в RSD.",
		},
	}

	for lang, want := range cases {
		t.Run(lang, func(t *testing.T) {
			localizer := bundle.NewLocalizer(lang)
			translations := map[string]string{
				"kpo.col_product":             want.product,
				"kpo.col_service":             want.service,
				"kpo.col_total":               want.total,
				"kpo.rsd_note":                want.note,
				"kpo.original_amount_tooltip": want.tooltip,
			}
			for key, expected := range translations {
				if got := localizer.T(key); got != expected {
					t.Errorf("%s = %q, want %q", key, got, expected)
				}
			}
		})
	}
}
