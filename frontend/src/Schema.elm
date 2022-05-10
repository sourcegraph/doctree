module Schema exposing (..)

import Json.Decode as Decode exposing (Decoder)
import Json.Decode.Pipeline as Pipeline


indexDecoder : Decoder Index
indexDecoder =
    Decode.succeed Index
        |> Pipeline.required "schemaVersion" Decode.string
        |> Pipeline.required "directory" Decode.string
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
        |> Pipeline.required "repository" Decode.string
        |> Pipeline.required "id" Decode.string
        |> Pipeline.required "version" Decode.string
        |> Pipeline.required "versionType" Decode.string
        |> Pipeline.required "pages" (Decode.list pageDecoder)


type alias Library =
    { -- Name of the library
      name : String
    , -- Repository the documentation lives in, a Git remote URL. e.g. "https://github.com/golang/go"
      -- or "git@github.com:golang/go"
      repository : String
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
