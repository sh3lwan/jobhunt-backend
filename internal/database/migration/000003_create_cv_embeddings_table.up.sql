CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE cv_embeddings
(
    cv_id BIGINT PRIMARY KEY REFERENCES cv_analyses (id),
    canonical_text TEXT,
    skills_text TEXT,
    responsibilities_text TEXT,
    canonical_text_embeddings VECTOR(768) DEFAULT NULL,
    skills_text_embeddings VECTOR(768) DEFAULT NULL,
    responsibilities_text_embeddings VECTOR(768) DEFAULT NULL,
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP DEFAULT now()
);
