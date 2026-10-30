package pipeline

// Block is one oar structure region.
type Block struct {
	Type    string    `json:"type"`
	Content string    `json:"content"`
	Order   int       `json:"order"`
	BBox    []float64 `json:"bbox"` // [x0,y0,x1,y1]
}

// Page is one page of structure blocks.
type Page struct {
	Blocks []Block
}

type item struct {
	text string
	typ  string
	em   float64
	w    float64
	x0   float64
	y0   float64
	y1   float64
	page int
}

type seg struct {
	kind   string // heading | para
	lvl    int
	text   string
	em     float64
	w      float64
	x0     float64
	y0     float64
	y1     float64
	series string
	num    int
	typ    string
}
