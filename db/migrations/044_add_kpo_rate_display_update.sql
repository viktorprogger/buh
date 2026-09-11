-- Publish the KPO currency/rate display improvement in all supported languages.
INSERT INTO app_updates (
    id, title_sr, body_sr, title_en, body_en, title_ru, body_ru, published_at
) VALUES (
    '00000000-0000-0000-0000-000000000045',
    'Побољшан приказ валуте и курса у КПО',
    '<p>У КПО је исправљен приказ избора валуте и тренутног курса НБС-а. Валута и курс су сада приказани јасније.</p>',
    'Improved currency and NBS rate display in KPO',
    '<p>Currency selection and the current NBS exchange rate in KPO are now displayed more clearly.</p>',
    'Улучшено отображение валюты и курса в КПО',
    '<p>В КПО исправлено отображение выбора валюты и текущего курса НБС. Теперь валюта и курс отображаются понятнее.</p>',
    '2026-09-01T00:00:00Z'
)
ON CONFLICT (id) DO NOTHING;
