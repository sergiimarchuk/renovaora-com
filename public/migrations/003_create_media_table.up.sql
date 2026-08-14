CREATE TABLE media (
    id SERIAL PRIMARY KEY,
    filename TEXT NOT NULL,
    path TEXT NOT NULL,
    alt_text TEXT,
    caption TEXT,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('page', 'article')),
    entity_id INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_media_entity ON media(entity_type, entity_id);
CREATE INDEX idx_media_deleted_at ON media(deleted_at);
