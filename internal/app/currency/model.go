package currency

type Currency struct {
	ID            int    `json:"id"`
	Code          string `json:"code"`
	Charactercode string `json:"charractercode"`
	Title         string `json:"title"`
}
