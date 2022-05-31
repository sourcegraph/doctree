module Schema exposing (..)

import Json.Decode as Decode exposing (Decoder)
import Json.Decode.Pipeline as Pipeline


indexDecoder : Decoder Index
indexDecoder =
    Decode.succeed Index
        |> Pipeline.required "schemaVersion" Decode.string
        |> Pipeline.required "directory" Decode.string
        |> Pipeline.required "gitRepository" Decode.string
        |> Pipeline.required "gitCommitID" Decode.string
        |> Pipeline.required "gitRefName" Decode.string
        |> Pipeline.required "createdAt" Decode.string
        |> Pipeline.required "numFiles" Decode.int
        |> Pipeline.required "numBytes" Decode.int
        |> Pipeline.required "durationSeconds" Decode.float
        |> Pipeline.required "language" languageDecoder
        |> Pipeline.required "libraries" (Decode.list libraryDecoder)


type alias Index =
    { -- The version of the doctree schema in use. Set this to the LatestVersion constant.
      schemaVersion : String
    , -- Directory that was indexed (absolute path.)
      directory : String
    , -- GitRepository is the normalized Git repository URI. e.g. "https://github.com/golang/go" or
      -- "git@github.com:golang/go" - the same value reported by `git config --get remote.origin.url`
      -- with `git@github.com:foo/bar` rewritten to `git://github.com/foo/bar`, credentials removed,
      -- any ".git" suffix removed, and any leading "/" prefix removed.
      --
      -- Empty string if the indexed directory was not a Git repository.
      gitRepository : String
    , -- GitCommitID is the SHA commit hash of the Git repository revision at the time of indexing, as
      -- reported by `git rev-parse HEAD`.
      --
      -- Empty string if the indexed directory was not a Git repository.
      gitCommitID : String
    , -- GitRefName is the current Git ref name (branch name, tag name, etc.) as reported by `git rev-parse --abbrev-ref HEAD`
      --
      -- Empty string if the indexed directory was not a Git repository.
      gitRefName : String
    , -- CreatedAt time of the index (RFC3339)
      createdAt : String
    , -- NumFiles indexed.
      numFiles : Int
    , -- NumBytes indexed.
      numBytes : Int
    , -- DurationSeconds is how long indexing took.
      durationSeconds : Float
    , -- Language name
      language : Language
    , -- Library documentation
      libraries : List Library
    }


languageDecoder : Decoder Language
languageDecoder =
    Decode.succeed Language
        |> Pipeline.required "title" Decode.string
        |> Pipeline.required "id" Decode.string


type alias Language =
    { -- Title of the language, e.g. "C++" or "Objective-C"
      title : String
    , -- ID of the language, e.g. "cpp", "objc". Lowercase.
      id : String
    }


libraryDecoder : Decoder Library
libraryDecoder =
    Decode.succeed Library
        |> Pipeline.required "name" Decode.string
        |> Pipeline.required "id" Decode.string
        |> Pipeline.required "version" Decode.string
        |> Pipeline.required "versionType" Decode.string
        |> Pipeline.required "pages" (Decode.list pageDecoder)


type alias Library =
    { -- Name of the library
      name : String
    , -- ID of this repository. Many languages have a unique identifier, for example in Java this may
      -- be "com.google.android.webview" in Python it may be the PyPi package name. For Rust, the
      -- Cargo crate name, etc.
      id : String
    , -- Version of the library
      version : String
    , -- Version string type, e.g. "semver", "commit"
      versionType : String
    , -- Pages of documentation for the library.
      pages : List Page
    }


pageDecoder : Decoder Page
pageDecoder =
    Decode.succeed Page
        |> Pipeline.required "path" Decode.string
        |> Pipeline.required "title" Decode.string
        |> Pipeline.required "detail" Decode.string
        |> Pipeline.required "searchKey" (Decode.list Decode.string)
        |> Pipeline.required "sections" (Decode.lazy (\_ -> sectionsDecoder))
        |> Pipeline.optional "subpages" (Decode.lazy (\_ -> pagesDecoder)) (Pages [])


type alias Page =
    { -- Path of the page relative to the library. This is the URL path and does not necessarily have
      -- to match up with filepaths.
      path : String
    , -- Title of the page.
      title : String
    , -- The detail
      detail : Markdown

    -- SearchKey describes a single string a user would type in to a search bar to find this
    -- page. For example, in Go this might be "net/http"
    -- This is a list of strings to diffentiate the different "parts" of the string, for Go it would
    -- actually be ["net", "/", "http"]. The search engine will do fuzzy prefix/suffix matching of
    -- each *part* of the key. For example, a query for "net" would be treated as "*net*".
    -- The key should aim to be unique within the scope of the directory and language that was
    -- indexed (you can imagine the key is prefixed with the language name and directory/repository
    -- name for you.)
    , searchKey : List String
    , -- Sections of the page.
      sections : Sections
    , -- Subpages of this one.
      subpages : Pages
    }


type Pages
    = Pages (List Page)


pagesDecoder =
    Decode.map Pages <| Decode.list (Decode.lazy (\_ -> pageDecoder))


sectionDecoder : Decoder Section
sectionDecoder =
    Decode.succeed Section
        |> Pipeline.required "id" Decode.string
        |> Pipeline.required "category" Decode.bool
        |> Pipeline.required "shortLabel" Decode.string
        |> Pipeline.required "label" Decode.string
        |> Pipeline.required "detail" Decode.string
        |> Pipeline.required "searchKey" (Decode.list Decode.string)
        |> Pipeline.optional "children" (Decode.lazy (\_ -> sectionsDecoder)) (Sections [])


type alias Section =
    { -- The ID of this section, used in the hyperlink to link to this section of the page.
      id : String
    , -- Category indicates if this section is just describing a category of children, for example
      -- if this section has the label "Functions" and Children are all of the functions in the
      -- library. This information is used to pick out key sections that should be shown in high-level
      -- navigation.
      category : Bool
    , -- ShortLabel is the shortest string that can describe this section relative to the parent. For
      -- example, in Go this may be `(r) GetName` as a reduced form of `func (r *Route) GetName`.
      shortLabel : String
    , -- The label of this section.
      label : Markdown
    , -- The detail
      detail : Markdown

    -- SearchKey describes a single string a user would type in to a search bar to find this
    -- section. For example, in Go this might be "net/http.Client.PostForm"
    --
    -- This is a list of strings to diffentiate the different "parts" of the string, for Go it would
    -- actually be ["net", "/", "http", ".", "Client", ".", "PostForm"]. The search engine will do
    -- fuzzy prefix/suffix matching of each *part* of the key. For example, a query for
    -- "net.PostForm" would be treated as "*net*.*PostForm*".
    --
    -- The key should aim to be unique within the scope of the directory and language that was
    -- indexed (you can imagine the key is prefixed with the language name and directory/repository
    -- name for you.)
    , searchKey : List String
    , -- Any children sections. For example, if this section represents a class the children could be
      -- the methods of the class and they would be rendered immediately below this section and
      -- indicated as being children of the parent section.
      children : Sections
    }


type Sections
    = Sections (List Section)


sectionsDecoder =
    Decode.map Sections <| Decode.list (Decode.lazy (\_ -> sectionDecoder))


type alias Markdown =
    String
