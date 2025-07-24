package models

// Some function signature with the output type, params, and documentation.
type Function struct {
	Name    string
	Params  string
	Results string
	FullSig string
	Doc     string
}

// A struct declaration, with fields and a documentation comment.
type Struct struct {
	Name   string
	Fields string
	Doc    string
}

// A top-level declared variable.
type Global struct {
	Name        string
	Declaration string
	Doc         string
}

// Data to be stored on a documentation template.
type PageData struct {
	PackageName string
	Functions   []Function
	Structs     []Struct
	Globals     []Global
	SubPackages []IndexEntry
}

// A single index entry for documentation root.
type IndexEntry struct {
	PackageName string
	DocFile     string
}
