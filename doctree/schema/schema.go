// Package schema describes the doctree schema, a standard JSON file format for describing library
// documentation.
//
// tree-sitter is used to emit documentation in this format, and the doctree frontend renders it.
//
// TODO: make SchemaVersion a semver type?
// TODO: make make Library.Version a type instead of two separate fields
package schema

// LatestVersion of the doctree schema (semver.)
const LatestVersion = "0.0.1"

// Index is the top-most data structure in the doctree schema.
type Index struct {
	// The version of the doctree schema in use. Set this to the LatestVersion constant.
	SchemaVersion string `json:"schemaVersion"`

	// Directory that was indexed (absolute path.)
	Directory string `json:"directory"`

	// CreatedAt time of the index (RFC3339)
	CreatedAt string `json:"createdAt"`

	// NumFiles indexed.
	NumFiles int `json:"numFiles"`

	// NumBytes indexed.
	NumBytes int `json:"numBytes"`

	// DurationSeconds is how long indexing took.
	DurationSeconds float64 `json:"durationSeconds"`

	// Language name.
	Language Language `json:"language"`

	// Library documentation.
	Library Library `json:"library"`
}

// Language name in canonical form, e.g. "Go", "Objective-C", etc.
type Language struct {
	// Title of the language, e.g. "C++" or "Objective-C"
	Title string `json:"title"`

	// ID of the language, e.g. "cpp", "objc". Lowercase.
	ID string `json:"id"`
}

// Language name constants.
var (
	LanguageC          = Language{Title: "C", ID: "c"}
	LanguageCpp        = Language{Title: "C++", ID: "cpp"}
	LanguageGo         = Language{Title: "Go", ID: "go"}
	LanguageJava       = Language{Title: "Java", ID: "java"}
	LanguageObjC       = Language{Title: "Objective-C", ID: "objc"}
	LanguagePython     = Language{Title: "Python", ID: "python"}
	LanguageTypeScript = Language{Title: "TypeScript", ID: "typescript"}
	LanguageZig        = Language{Title: "Zig", ID: "zig"}
)

// Library documentation, the most top-level data structure. Represents a code library / a logical
// unit of code typically distributed by package managers.
type Library struct {
	// Name of the library
	Name string `json:"name"`

	// Repository the documentation lives in, a Git remote URL. e.g. "https://github.com/golang/go"
	// or "git@github.com:golang/go"
	Repository string `json:"repository"`

	// ID of this repository. Many languages have a unique identifier, for example in Java this may
	// be "com.google.android.webview" in Python it may be the PyPi package name. For Rust, the
	// Cargo crate name, etc.
	ID string `json:"id"`

	// Version of the library
	Version string `json:"version"`

	// Version string type, e.g. "semver", "commit"
	VersionType string `json:"versionType"`

	// Pages of documentation for the library.
	Pages []Page `json:"pages"`
}

// Page is a single page of documentation, and typically gets rendered as a single page in the
// browser.
type Page struct {
	// Path of the page relative to the library. This is the URL path and does not necessarily have
	// to match up with filepaths.
	Path string `json:"path"`

	// Title of the page.
	Title string `json:"title"`

	// The detail
	Detail Markdown `json:"detail"`

	// Sections on the page.
	Sections []Section `json:"sections"`
}

// Section represents a single section of documentation on a page. These give you building blocks
// to arrange the page how you see fit. For example, in Go maybe you want documentation to be
// structured as:
//
// * Overview
// * Constants
// * Variables
// * Functions
//   * func SetURLVars
// * Types
//   * type Route
//     * (r) GetName
//
// Each of these bullet points in the list above is a Section!
type Section struct {
	// The ID of this section, used in the hyperlink to link to this section of the page.
	ID string `json:"id"`

	// ShortLabel is the shortest string that can describe this section relative to the parent. For
	// example, in Go this may be `(r) GetName` as a reduced form of `func (r *Route) GetName`.
	ShortLabel string `json:"shortLabel"`

	// The label of this section.
	Label Markdown `json:"label"`

	// The detail
	Detail Markdown `json:"detail"`

	// Any children sections. For example, if this section represents a class the children could be
	// the methods of the class and they would be rendered immediately below this section and
	// indicated as being children of the parent section.
	Children []Section `json:"children"`
}

// Markdown text.
type Markdown string
