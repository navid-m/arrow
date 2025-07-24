package models

type Function struct {
	Name    string
	Params  string
	Results string
	FullSig string
	Doc     string
}

type Struct struct {
	Name   string
	Fields string
	Doc    string
}

type Global struct {
	Name        string
	Declaration string
	Doc         string
}

type PageData struct {
	PackageName string
	Functions   []Function
	Structs     []Struct
	Globals     []Global
	SubPackages []IndexEntry
}

type IndexEntry struct {
	PackageName string
	DocFile     string
}
