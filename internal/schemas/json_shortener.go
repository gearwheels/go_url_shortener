package schemasshortener

type RequestSchema struct {
	URL string `json:"url"`
}

type ResponseSchema struct {
	Result string `json:"result"`
}
