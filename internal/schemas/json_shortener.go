// Package schemasshortener содержит структуры запросов и ответов JSON API.
package schemasshortener

// RequestSchema — тело POST /api/shorten.
type RequestSchema struct {
	URL string `json:"url"`
}

// ResponseSchema — ответ POST /api/shorten.
type ResponseSchema struct {
	Result string `json:"result"`
}

// RequestBatchURLSchema — один элемент пакетного запроса POST /api/shorten/batch.
type RequestBatchURLSchema struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// ResponseBatchURLSchema — один элемент ответа POST /api/shorten/batch.
type ResponseBatchURLSchema struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}
