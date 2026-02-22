package exchange

type Instrument struct {
	Symbol   string
	ISIN     string
	Exchange string
	Name     string
	Type     string // "equity", "indices", "commodity"
}
