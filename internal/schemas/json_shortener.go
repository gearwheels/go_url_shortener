package schemasShortener

type RequestSchema struct {
	Url string `json:"url"`
}

type ResponseSchema struct {
	Result string `json:"result"`
}
