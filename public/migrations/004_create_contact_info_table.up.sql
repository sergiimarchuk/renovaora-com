CREATE TABLE contact_info (
    id SERIAL PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

INSERT INTO contact_info (key, value) VALUES 
    ('phone', ''),
    ('email', ''),
    ('address', ''),
    ('hours', '');
