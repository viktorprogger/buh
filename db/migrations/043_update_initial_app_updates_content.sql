-- Render the initial production updates as semantic HTML and publish them on
-- the intended date. Keep this migration limited to the two production rows.
UPDATE app_updates
SET body_sr = CASE id
        WHEN '00000000-0000-0000-0000-000000000042' THEN '<p>У КПО књизи сада можете да изаберете валуту прихода. Износ у RSD се израчунава по историјском средњем курсу НБС-а за датум ставке.</p>'
        WHEN '00000000-0000-0000-0000-000000000043' THEN '<p>На листи уплатница сада можете означити више ставки и преузети их заједно у једном ZIP архиву.</p>'
    END,
    body_en = CASE id
        WHEN '00000000-0000-0000-0000-000000000042' THEN '<p>You can now choose the income currency directly in the KPO book. The RSD amount is calculated using the NBS historical middle rate for the entry date.</p>'
        WHEN '00000000-0000-0000-0000-000000000043' THEN '<p>On the payment slips list, you can now select several slips and download them together in a single ZIP archive.</p>'
    END,
    body_ru = CASE id
        WHEN '00000000-0000-0000-0000-000000000042' THEN '<p>Теперь в книге КПО можно выбрать валюту дохода. Сумма в RSD рассчитывается по историческому среднему курсу НБС на дату записи.</p>'
        WHEN '00000000-0000-0000-0000-000000000043' THEN '<p>В списке платёжных поручений теперь можно выбрать несколько записей и скачать их одним ZIP-архивом.</p>'
    END,
    published_at = '2026-08-23T00:00:00Z'
WHERE id IN (
    '00000000-0000-0000-0000-000000000042',
    '00000000-0000-0000-0000-000000000043'
);
