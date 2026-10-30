package markdown

// Block is the intermediate representation before GFM rendering.
type Block struct {
	Type  string // heading | paragraph | table | list
	Level int    // heading level 1-6
	Text  string
	Page  int
}
