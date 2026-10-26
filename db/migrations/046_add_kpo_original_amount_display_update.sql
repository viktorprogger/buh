-- Publish the improved foreign-currency income display in all supported languages.
INSERT INTO app_updates (
    id, title_sr, body_sr, title_en, body_en, title_ru, body_ru, published_at
) VALUES (
    '00000000-0000-0000-0000-000000000047',
    'Побољшан приказ прихода у страној валути у КПО',
    '<p>У КПО је побољшан приказ прихода у страној валути. Уз износ у RSD сада се јасно приказују и оригинални износ и валута, док се у PDF-у КПО и даље приказује само износ у RSD.</p>',
    'Improved display of foreign-currency income in KPO',
    '<p>Foreign-currency income is now displayed more clearly in KPO. The original amount and currency are shown alongside the RSD amount, while the KPO PDF continues to include only the RSD amount.</p>',
    'Улучшено отображение доходов в иностранной валюте в КПО',
    '<p>В КПО улучшено отображение доходов в иностранной валюте. Рядом с суммой в RSD теперь отображаются исходная сумма и валюта, а в PDF КПО по-прежнему указывается только сумма в RSD.</p>',
    '2026-10-26T00:00:00Z'
)
ON CONFLICT (id) DO NOTHING;
