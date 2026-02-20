package schemasshortener

type RequestSchema struct {
	URL string `json:"url"`
}

type ResponseSchema struct {
	Result string `json:"result"`
}

type RequestBatchURLSchema struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL string `json:"original_url"`
}

type ResponseBatchURLSchema struct {
	CorrelationID string `json:"correlation_id"`
	ShortUrl string `json:"short_url"`
}