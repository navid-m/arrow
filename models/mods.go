package models

// Represents a function or method declaration
type Function struct {
	Name     string
	Params   string
	Results  string
	FullSig  string
	Doc      string
	Receiver string
	IsMethod bool
}

// Represents a struct type declaration
type Struct struct {
	Name   string
	Fields string
	Doc    string
	Kind   string
}

// Represents an interface type declaration
type Interface struct {
	Name    string
	Methods string
	Doc     string
}

// Represents a type alias or type definition
type TypeAlias struct {
	Name string
	Type string
	Doc  string
}

// Represents a top-level variable or constant
//
// Kind can be var or const
type Global struct {
	Name        string
	Declaration string
	Doc         string
	Kind        string
}

// Represents an import declaration
type Import struct {
	Name string
	Path string
}

// Contains all data for a documentation page
type PageData struct {
	PackageName string
	Functions   []Function
	Structs     []Struct
	Interfaces  []Interface
	Types       []TypeAlias
	Globals     []Global
	Imports     []Import
	SubPackages []IndexEntry
}

// Represents a single entry in the documentation index
type IndexEntry struct {
	PackageName string
	DocFile     string
}
