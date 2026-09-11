CREATE TABLE IF NOT EXISTS app_updates (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title_sr     TEXT NOT NULL,
    body_sr      TEXT NOT NULL,
    title_en     TEXT NOT NULL,
    body_en      TEXT NOT NULL,
    title_ru     TEXT NOT NULL,
    body_ru      TEXT NOT NULL,
    published_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS app_updates_published_at_idx
    ON app_updates (published_at DESC);

CREATE TABLE IF NOT EXISTS app_update_reads (
    update_id UUID NOT NULL REFERENCES app_updates(id) ON DELETE CASCADE,
    user_type TEXT NOT NULL CHECK (user_type IN ('accountant', 'entrepreneur')),
    user_id   UUID NOT NULL,
    read_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (update_id, user_type, user_id)
);
CREATE INDEX IF NOT EXISTS app_update_reads_user_idx
    ON app_update_reads (user_type, user_id);

-- Manual initial entries. Future entries should be added in a follow-up
-- migration with explicit publication timestamps and all three translations.
INSERT INTO app_updates (
    id, title_sr, body_sr, title_en, body_en, title_ru, body_ru, published_at
) VALUES
(
    '00000000-0000-0000-0000-000000000042',
    'Избор валуте у КПО',
    'У КПО књизи сада можете да изаберете валуту прихода. Износ у RSD се израчунава по историјском средњем курсу НБС-а за датум ставке.',
    'Currency selection in the KPO book',
    'You can now choose the income currency directly in the KPO book. The RSD amount is calculated using the NBS historical middle rate for the entry date.',
    'Выбор валюты в КПО',
    'Теперь в книге КПО можно выбрать валюту дохода. Сумма в RSD рассчитывается по историческому среднему курсу НБС на дату записи.',
    '2026-08-17T00:00:00Z'
),
(
    '00000000-0000-0000-0000-000000000043',
    'Преузимање више уплатница одједном',
    'На листи уплатница сада можете означити више ставки и преузети их заједно у једном ZIP архиву.',
    'Download multiple payment slips at once',
    'On the payment slips list, you can now select several slips and download them together in a single ZIP archive.',
    'Скачивание нескольких платёжных поручений',
    'В списке платёжных поручений теперь можно выбрать несколько записей и скачать их одним ZIP-архивом.',
    '2026-08-18T00:00:00Z'
)
ON CONFLICT (id) DO NOTHING;
