-- +goose Up
INSERT INTO business_categories (name, slug) VALUES
    ('Restaurants', 'restaurants'),
    ('Hotels', 'hotels'),
    ('Medical', 'medical'),
    ('Education', 'education'),
    ('Shopping', 'shopping'),
    ('Automobile', 'automobile'),
    ('Services', 'services'),
    ('Finance', 'finance'),
    ('Real Estate', 'real-estate'),
    ('Other', 'other')
ON CONFLICT (slug) DO NOTHING;

-- +goose Down
DELETE FROM business_categories WHERE slug IN (
    'restaurants', 'hotels', 'medical', 'education', 'shopping',
    'automobile', 'services', 'finance', 'real-estate', 'other'
);