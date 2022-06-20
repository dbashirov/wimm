package currency

type Currency struct {
	Id            int    `json:"id"`
	Code          string `json:"code"`
	Charactercode string `json:"charractercode"`
	Title         string `json:"title"`
}
