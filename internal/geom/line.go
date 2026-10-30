package geom

// Line is a geometry-aware text row (pdf-md Line model).
type Line struct {
	Text       string
	Page       int
	X0, Y0     float64
	X1, Y1     float64
	PageWidth  float64
	PageHeight float64
	Source     string  // "text" | "ocr"
	FontSize   float64 // 0 = unknown; often estimated from line height
	Bold       bool    // true when font flags / name indicate bold
}

func (l Line) XCenter() float64 { return (l.X0 + l.X1) / 2 }

func (l Line) Height() float64 {
	h := l.Y1 - l.Y0
	if h > 0 {
		return h
	}
	return 0
}

func (l Line) Em() float64 {
	if l.FontSize > 0 {
		return l.FontSize
	}
	if h := l.Height(); h > 0 {
		return h
	}
	return 12
}

// MergeBBox returns a line covering both boxes with concatenated text.
func MergeBBox(left, right Line, text string) Line {
	font := left.FontSize
	if font <= 0 {
		font = right.FontSize
	}
	return Line{
		Text:       text,
		Page:       left.Page,
		X0:         min(left.X0, right.X0),
		Y0:         min(left.Y0, right.Y0),
		X1:         max(left.X1, right.X1),
		Y1:         max(left.Y1, right.Y1),
		PageWidth:  left.PageWidth,
		PageHeight: left.PageHeight,
		Source:     left.Source,
		FontSize:   font,
		Bold:       left.Bold || right.Bold,
	}
}

func Texts(lines []Line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.Text
	}
	return out
}
