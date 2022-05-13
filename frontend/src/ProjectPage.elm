module ProjectPage exposing (Model, Msg, init, page, subscriptions, update, view)

import APISchema
import Browser
import Browser.Dom
import Dict exposing (keys)
import Effect exposing (Effect)
import Element as E
import Element.Border as Border
import Element.Font as Font
import Element.Region as Region
import Gen.Params.NotFound exposing (Params)
import Html exposing (Html)
import Html.Attributes exposing (style)
import Markdown
import Page
import Process
import Request
import Schema
import Shared
import Style
import Task
import Url.Builder
import Url.Parser
import Url.Parser.Query
import Util exposing (httpErrorToString)
import View exposing (View)


page : Shared.Model -> Request.With Params -> Page.With Model Msg
page shared req =
    let
        rawProjectURI =
            String.dropLeft (String.length "/") req.url.path

        projectURI =
            parseProjectURI rawProjectURI

        urlParams =
            decodeUrlParams req.query
    in
    Page.advanced
        { init = init projectURI urlParams.id
        , update = update
        , view = view shared projectURI
        , subscriptions = subscriptions
        }


type alias UrlParams =
    { id : Maybe String
    }


decodeUrlParams : Dict.Dict String String -> UrlParams
decodeUrlParams params =
    { id = Dict.get "id" params
    }


type ProjectURI
    = Name String
    | NameLanguage String String
    | NameLanguagePage String String String
    | NameLanguagePageSection String String String String


parseProjectURI : String -> Maybe ProjectURI
parseProjectURI uri =
    case String.split "/-/" uri of
        name :: language :: docPage :: section :: _ ->
            Just (NameLanguagePageSection name language docPage section)

        name :: language :: docPage :: _ ->
            Just (NameLanguagePage name language docPage)

        name :: language :: _ ->
            Just (NameLanguage name language)

        name :: _ ->
            Just (Name name)

        _ ->
            Nothing



-- INIT


type alias Model =
    {}


init : Maybe ProjectURI -> Maybe String -> ( Model, Effect Msg )
init projectURI maybeID =
    ( {}
    , case projectURI of
        Just uri ->
            Effect.batch
                [ Effect.fromShared (Shared.GetProject (projectURIName uri))
                , case maybeID of
                    Just id ->
                        Effect.fromCmd (scrollIntoViewHack id)

                    Nothing ->
                        Effect.none
                ]

        Nothing ->
            Effect.none
    )


{-| HACK to workaround <https://elmlang.slack.com/archives/C192T0Q1E/p1652407639492269>
maybe a bug in Elm? Without this, Browser.Dom.getElement doesn't work because the
page isn't rendered yet.
-}
scrollIntoViewHack : String -> Cmd Msg
scrollIntoViewHack id =
    Cmd.batch
        [ scrollIntoView 100 id
        , scrollIntoView 250 id
        , scrollIntoView 500 id
        , scrollIntoView 1000 id
        , scrollIntoView 3000 id
        ]


scrollIntoView : Float -> String -> Cmd Msg
scrollIntoView sleepTime id =
    Process.sleep sleepTime
        |> Task.andThen (\_ -> Browser.Dom.getElement id)
        |> Task.andThen (\info -> Browser.Dom.setViewport 0 info.element.y)
        |> Task.attempt (\_ -> NoOp)


projectURIName : ProjectURI -> String
projectURIName projectURI =
    case projectURI of
        Name v ->
            v

        NameLanguage v _ ->
            v

        NameLanguagePage v _ _ ->
            v

        NameLanguagePageSection v _ _ _ ->
            v



-- UPDATE


type Msg
    = NoOp


update : Msg -> Model -> ( Model, Effect Msg )
update msg model =
    case msg of
        NoOp ->
            ( model, Effect.none )



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none



-- VIEW


view : Shared.Model -> Maybe ProjectURI -> Model -> View Msg
view shared projectURI model =
    { title = "TODO"
    , body =
        [ case shared.projectIndexes of
            Just response ->
                case response of
                    Ok projectIndexes ->
                        case projectURI of
                            Just uri ->
                                case uri of
                                    NameLanguagePageSection name language docPage section ->
                                        -- viewNameLanguagePageSection model name language docPage section
                                        viewNameLanguagePage model projectIndexes name language docPage

                                    NameLanguagePage name language docPage ->
                                        viewNameLanguagePage model projectIndexes name language docPage

                                    NameLanguage name language ->
                                        viewNameLanguage model projectIndexes name language

                                    Name name ->
                                        viewNameLanguage model projectIndexes name ""

                            Nothing ->
                                E.layout Style.layout (E.text "loading..")

                    Err err ->
                        E.layout Style.layout (E.text (httpErrorToString err))

            Nothing ->
                E.layout Style.layout (E.text "loading..")
        ]
    }


viewName : Model -> APISchema.ProjectIndexes -> String -> Html Msg
viewName model projectIndexes projectName =
    E.layout Style.layout
        (E.column []
            (List.map
                (\language ->
                    E.link []
                        { url =
                            String.concat
                                [ Url.Builder.absolute [ projectName, "-", language ] [] ]
                        , label = E.text language
                        }
                )
                (Dict.keys projectIndexes)
            )
        )


viewNameLanguage : Model -> APISchema.ProjectIndexes -> String -> String -> Html Msg
viewNameLanguage model projectIndexes projectName language =
    let
        firstLanguage =
            Maybe.withDefault "" (List.head (keys projectIndexes))

        selectedLanguage =
            if language == "" then
                firstLanguage

            else
                language

        indexLookup =
            Dict.get selectedLanguage projectIndexes
    in
    case indexLookup of
        Just index ->
            let
                listHead =
                    List.head index.libraries
            in
            case listHead of
                Just library ->
                    E.layout (List.concat [ Style.layout, [ E.width E.fill ] ])
                        (E.column [ E.centerX, E.paddingXY 0 32 ]
                            [ E.row []
                                [ E.link [] { url = "/", label = logo }
                                , E.el [ Region.heading 1, Font.size 24 ] (E.text (String.concat [ " / ", projectName ]))
                                ]
                            , E.row [ E.width E.fill, E.paddingXY 0 64 ]
                                -- TODO: Should UI sort pages, or indexers themselves decide order? Probably the latter?
                                [ E.column [ E.width (E.fillPortion 1) ]
                                    (List.map
                                        (\docPage ->
                                            E.link [ E.width (E.fillPortion 1) ]
                                                { url =
                                                    Url.Builder.absolute [ projectName, "-", selectedLanguage, "-", docPage.path ] []
                                                , label = E.el [ Font.underline ] (E.text docPage.path)
                                                }
                                        )
                                        (List.sortBy .path library.pages)
                                    )
                                , E.column [ E.width (E.fillPortion 1) ]
                                    (List.map
                                        (\docPage ->
                                            E.link [ E.width (E.fillPortion 1) ]
                                                { url =
                                                    Url.Builder.absolute [ projectName, "-", selectedLanguage, "-", docPage.path ] []
                                                , label = E.el [ Font.underline ] (E.text docPage.title)
                                                }
                                        )
                                        (List.sortBy .path library.pages)
                                    )
                                ]
                            ]
                        )

                Nothing ->
                    E.layout Style.layout (E.text "error: invalid index: must have at least on library")

        Nothing ->
            E.layout Style.layout (E.text "language not found")


viewNameLanguagePage : Model -> APISchema.ProjectIndexes -> String -> String -> String -> Html Msg
viewNameLanguagePage model projectIndexes projectName language targetPagePath =
    let
        pageLookup =
            Dict.get language projectIndexes
                |> Maybe.andThen (\index -> List.head index.libraries)
                |> Maybe.andThen (\library -> Just (List.filter (\docPage -> docPage.path == targetPagePath) library.pages))
                |> Maybe.andThen (\pages -> List.head pages)
    in
    case pageLookup of
        Just docPage ->
            let
                subpages =
                    case docPage.subpages of
                        Schema.Pages v ->
                            v
            in
            E.layout (List.concat [ Style.layout, [ E.width E.fill ] ])
                (E.column [ maxWidth, E.centerX ]
                    [ E.column [ maxWidth ]
                        [ E.wrappedRow [ E.paddingXY 0 32 ]
                            [ E.link [] { url = "/", label = logo }
                            , E.link [ Region.heading 1, Font.size 20 ] { url = Url.Builder.absolute [ projectName ] [], label = E.text (String.concat [ " / ", projectName ]) }
                            , E.el [ Region.heading 1, Font.size 20 ] (E.text (String.concat [ " : ", String.toLower docPage.title ]))
                            ]
                        , Style.h1 [] (E.text docPage.title)
                        , E.el [ E.paddingXY 0 16 ] (Markdown.render docPage.detail)
                        , if List.length subpages > 0 then
                            E.column []
                                (List.concat
                                    [ [ Style.h2 [] (E.text "Subpages") ]
                                    , List.map (\subPage -> E.link [] { url = subPage.path, label = E.text subPage.title }) subpages
                                    ]
                                )

                          else
                            E.none
                        ]
                    , renderSections docPage.sections
                    ]
                )

        Nothing ->
            E.layout Style.layout (E.text "page not found")


maxWidth =
    E.width (E.fill |> E.maximum 1000)


renderSections : Schema.Sections -> E.Element Msg
renderSections sections =
    let
        list =
            case sections of
                Schema.Sections v ->
                    v
    in
    E.column [ maxWidth, E.paddingXY 0 16 ]
        (List.map (\section -> renderSection section) list)


renderSection : Schema.Section -> E.Element Msg
renderSection section =
    E.column []
        [ E.column [ maxWidth ]
            [ if section.category then
                Style.h2 [ E.paddingXY 0 8, E.htmlAttribute (Html.Attributes.id section.id) ]
                    (E.text (String.concat [ "# ", section.label ]))

              else
                Style.h3 [ E.paddingXY 0 8, E.htmlAttribute (Html.Attributes.id section.id) ]
                    (E.text (String.concat [ "# ", section.label ]))
            , if section.detail == "" then
                E.none

              else
                E.el
                    [ Border.color (E.rgb255 210 210 210)
                    , Border.widthEach { top = 0, left = 6, bottom = 0, right = 0 }
                    , E.paddingXY 16 16
                    ]
                    (Markdown.render section.detail)
            ]
        , E.el [] (renderSections section.children)
        ]


logo =
    E.row [ E.centerX ]
        [ E.image
            [ E.width (E.px 40)
            , E.paddingEach { top = 0, right = 45, bottom = 0, left = 0 }
            ]
            { src = "/mascot.svg", description = "cute computer / doctree mascot" }
        , E.el [ Font.size 32, Font.bold ] (E.text "doctree")
        ]
