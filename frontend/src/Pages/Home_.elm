module Pages.Home_ exposing (Model, Msg, page)

import Element as E
import Element.Font as Font
import Element.Lazy
import Gen.Params.Home_ exposing (Params)
import Http
import Json.Decode as D
import Page
import Request
import Search
import Shared
import Style
import Util exposing (httpErrorToString)
import View exposing (View)


page : Shared.Model -> Request.With Params -> Page.With Model Msg
page shared req =
    Page.element
        { init = init
        , update = update
        , view = view shared.flags.cloudMode
        , subscriptions = subscriptions
        }



-- INIT


type alias Model =
    { list : Maybe (Result Http.Error (List String))
    , search : Search.Model
    }


init : ( Model, Cmd Msg )
init =
    let
        ( searchModel, searchCmd ) =
            Search.init
    in
    ( { list = Nothing
      , search = searchModel
      }
    , Cmd.batch
        [ fetchList
        , Cmd.map (\v -> SearchMsg v) searchCmd
        ]
    )



-- UPDATE


type Msg
    = GotList (Result Http.Error (List String))
    | SearchMsg Search.Msg


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        GotList list ->
            ( { model | list = Just list }, Cmd.none )

        SearchMsg searchMsg ->
            let
                ( searchModel, searchCmd ) =
                    Search.update searchMsg model.search
            in
            ( { model | search = searchModel }
            , Cmd.map (\v -> SearchMsg v) searchCmd
            )


fetchList : Cmd Msg
fetchList =
    Http.get
        { url = "/api/list"
        , expect = Http.expectJson GotList (D.list D.string)
        }



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none



-- VIEW


view : Bool -> Model -> View Msg
view cloudMode model =
    { title = "doctree"
    , body =
        [ E.layout (List.concat [ Style.layout, [ E.width E.fill ] ])
            (case model.list of
                Just response ->
                    case response of
                        Ok list ->
                            E.column [ E.centerX, E.width (E.fill |> E.maximum 700), E.paddingXY 0 64 ]
                                [ logo
                                , E.map (\v -> SearchMsg v) Search.searchInput
                                , if model.search.query /= "" then
                                    Element.Lazy.lazy
                                        (\results -> E.map (\v -> SearchMsg v) (Search.searchResults results))
                                        model.search.results

                                  else if cloudMode then
                                    E.column [ E.centerX ]
                                        [ Style.h2 [ E.paddingXY 0 32 ] (E.text "# Try doctree (sample projects)")
                                        , E.column [] (projectsList list)
                                        , Style.h2 [ E.paddingEach { top = 32, right = 0, bottom = 0, left = 0 } ]
                                            (E.text "# Add your repository to doctree.org")
                                        , Style.paragraph [ E.paddingEach { top = 32, right = 0, bottom = 0, left = 0 } ]
                                            [ E.text "(coming soon)"
                                            ]
                                        , Style.h2 [ E.paddingEach { top = 32, right = 0, bottom = 0, left = 0 } ]
                                            (E.text "# About doctree")
                                        , Style.h3 [ E.paddingEach { top = 32, right = 0, bottom = 0, left = 0 } ]
                                            (E.text "# 100% open-source library docs tool for every language")
                                        , Style.paragraph [ E.paddingEach { top = 16, right = 0, bottom = 0, left = 0 } ]
                                            [ E.text "Available "
                                            , E.link [ Font.underline ] { url = "https://github.com/sourcegraph/doctree", label = E.text "on GitHub" }
                                            , E.text ", doctree provides first-class library documentation for every language (based on tree-sitter), with symbol search & more. If connected to Sourcegraph, it can automatically surface real-world usage examples."
                                            ]
                                        , Style.h3 [ E.paddingEach { top = 32, right = 0, bottom = 0, left = 0 } ] (E.text "# Run locally, self-host, or use doctree.org")
                                        , Style.paragraph [ E.paddingEach { top = 16, right = 0, bottom = 0, left = 0 } ]
                                            [ E.text "doctree is a single binary, lightweight, and designed to run on your local machine. It can be self-hosted, and used via doctree.org with any GitHub repository. "
                                            , E.link [ Font.underline ] { url = "https://github.com/sourcegraph/doctree#installation", label = E.text "installation instructions" }
                                            ]
                                        , Style.h3 [ E.paddingEach { top = 32, right = 0, bottom = 0, left = 0 } ]
                                            (E.text "# Experimental! Early stages!")
                                        , Style.paragraph [ E.paddingEach { top = 16, right = 0, bottom = 0, left = 0 } ]
                                            [ E.text "Extremely early stages, we're working on adding more languages, polishing the experience, and adding usage examples. It's all very early and not yet ready for production use, please bear with us!"
                                            ]
                                        , Style.paragraph [ E.paddingEach { top = 16, right = 0, bottom = 0, left = 0 } ]
                                            [ E.text "Please see "
                                            , E.link [ Font.underline ] { url = "https://github.com/sourcegraph/doctree/issues/27", label = E.text "the v1.0 roadmap" }
                                            , E.text " for more, ideas welcome!"
                                            ]
                                        , Style.h3 [ E.paddingEach { top = 32, right = 0, bottom = 0, left = 0 } ]
                                            (E.text "# Join us on Discord")
                                        , Style.paragraph [ E.paddingEach { top = 16, right = 0, bottom = 0, left = 0 } ]
                                            [ E.text "If you think what we're building is a good idea, we'd love to hear your thoughts! "
                                            , E.link [ Font.underline ] { url = "https://discord.gg/vqsBW8m5Y8", label = E.text "Discord invite" }
                                            ]
                                        , Style.h3 [ E.paddingEach { top = 32, right = 0, bottom = 0, left = 0 } ]
                                            (E.text "# Language support")
                                        , Style.paragraph [ E.paddingEach { top = 16, right = 0, bottom = 0, left = 0 } ]
                                            [ E.text "Adding support for more languages is easy. To request support for a language "
                                            , E.link [ Font.underline ] { url = "https://github.com/sourcegraph/doctree/issues/10", label = E.text "comment on this issue" }
                                            , E.text "!"
                                            ]
                                        , E.table [ E.paddingEach { top = 16, right = 0, bottom = 0, left = 0 } ]
                                            { data = supportedLanguages
                                            , columns =
                                                [ { header = Style.tableHeader (E.text "language")
                                                  , width = E.fill
                                                  , view = \lang -> Style.tableCell (E.text lang.name)
                                                  }
                                                , { header = Style.tableHeader (E.text "functions")
                                                  , width = E.fill
                                                  , view = \lang -> Style.tableCell (E.text lang.functions)
                                                  }
                                                , { header = Style.tableHeader (E.text "methods")
                                                  , width = E.fill
                                                  , view = \lang -> Style.tableCell (E.text lang.methods)
                                                  }
                                                , { header = Style.tableHeader (E.text "consts/vars")
                                                  , width = E.fill
                                                  , view = \lang -> Style.tableCell (E.text lang.constsVars)
                                                  }
                                                , { header = Style.tableHeader (E.text "search")
                                                  , width = E.fill
                                                  , view = \lang -> Style.tableCell (E.text lang.search)
                                                  }
                                                , { header = Style.tableHeader (E.text "usage examples")
                                                  , width = E.fill
                                                  , view = \lang -> Style.tableCell (E.text lang.usageExamples)
                                                  }
                                                , { header = Style.tableHeader (E.text "code intel")
                                                  , width = E.fill
                                                  , view = \lang -> Style.tableCell (E.text lang.codeIntel)
                                                  }
                                                ]
                                            }
                                        ]

                                  else
                                    E.column [ E.centerX ]
                                        [ Style.h2 [ E.paddingXY 0 32 ] (E.text "# Your projects")
                                        , E.column [] (projectsList list)
                                        , Style.h2 [ E.paddingXY 0 32 ] (E.text "# Index a project")
                                        , E.row [ Font.size 16 ]
                                            [ E.text "$ "
                                            , E.text "doctree index ."
                                            ]
                                        ]
                                ]

                        Err err ->
                            E.text (httpErrorToString err)

                Nothing ->
                    E.text "loading.."
            )
        ]
    }


type alias SupportedLanguage =
    { name : String
    , functions : String
    , methods : String
    , constsVars : String
    , search : String
    , usageExamples : String
    , codeIntel : String
    }


supportedLanguages : List SupportedLanguage
supportedLanguages =
    [ { name = "Go"
      , functions = "✅"
      , methods = "❌"
      , constsVars = "❌"
      , search = "✅"
      , usageExamples = "❌"
      , codeIntel = "❌"
      }
    , { name = "Python"
      , functions = "✅"
      , methods = "❌"
      , constsVars = "❌"
      , search = "✅"
      , usageExamples = "❌"
      , codeIntel = "❌"
      }
    , { name = "Zig"
      , functions = "✅"
      , methods = "partial"
      , constsVars = "❌"
      , search = "✅"
      , usageExamples = "❌"
      , codeIntel = "❌"
      }
    , { name = "Markdown"
      , functions = "n/a"
      , methods = "n/a"
      , constsVars = "❌"
      , search = "✅"
      , usageExamples = "❌"
      , codeIntel = "❌"
      }
    ]


logo =
    E.row [ E.centerX ]
        [ E.image
            [ E.width (E.px 120)
            , E.paddingEach { top = 0, right = 140, bottom = 0, left = 0 }
            ]
            { src = "/mascot.svg", description = "cute computer / doctree mascot" }
        , E.column []
            [ E.el [ Font.size 16, Font.bold, E.alignRight ] (E.text "v0.1")
            , E.el [ Font.size 64, Font.bold ] (E.text "doctree")
            , E.el [ Font.semiBold ] (E.text "documentation for every language")
            ]
        ]


projectsList list =
    List.map
        (\projectName ->
            E.link [ E.paddingXY 0 4 ]
                { url = projectName
                , label =
                    E.row []
                        [ E.text "• "
                        , E.el [ Font.underline ] (E.text projectName)
                        ]
                }
        )
        list
