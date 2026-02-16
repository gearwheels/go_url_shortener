-- migrations/000001_url_shortener.up.sql
-- Создание таблицы для сокращенных ссылок 
CREATE TABLE urls (
    id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    url VARCHAR(255) UNIQUE NOT NULL,
    short_url VARCHAR(255) UNIQUE NOT NULL
);

-- Базовый индекс для поиска по url
CREATE INDEX idx_url ON urls(url);

-- Индекс для поиска по short_url
CREATE INDEX idx_short_url ON urls(short_url); 