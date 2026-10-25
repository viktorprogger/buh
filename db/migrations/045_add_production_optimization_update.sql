-- Publish the production optimization update in all supported languages.
INSERT INTO app_updates (
    id, title_sr, body_sr, title_en, body_en, title_ru, body_ru, published_at
) VALUES (
    '00000000-0000-0000-0000-000000000046',
    'Техничка оптимизација апликације',
    '<p>Апликација је технички оптимизована. Функције и изглед су остали непромењени, а систем сада ради брже и има бољи одзив. Уживајте у раду!</p>',
    'Technical application optimization',
    '<p>The application has been technically optimized. The features and interface remain unchanged, while the system now runs faster and responds more quickly. Enjoy using the app!</p>',
    'Техническая оптимизация приложения',
    '<p>Мы провели техническую оптимизацию приложения. Функции и интерфейс остались прежними, а система теперь работает быстрее и отзывчивее. Приятной работы!</p>',
    '2026-10-25T00:00:00Z'
)
ON CONFLICT (id) DO NOTHING;
