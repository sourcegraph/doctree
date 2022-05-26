module ProjectPage exposing (Model, Msg, init, page, subscriptions, update, view)

import APISchema
import Browser.Dom
import Browser.Navigation
import Dict exposing (keys)
import Effect exposing (Effect)
import Element as E
import Element.Background as Background
import Element.Border as Border
import Element.Font as Font
import Element.Lazy
import Element.Region as Region
import Gen.Params.NotFound exposing (Params)
import Html exposing (Html)
import Html.Attributes
import Http
import Json.Decode
import Markdown
import Page
import Ports
import Process
import Request
import Schema
import Search
import Shared
import Style
import Task
import Url.Builder
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
        , update = update projectURI req
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
            Just
                (NameLanguagePageSection name
                    language
                    (if docPage == "" then
                        "/"

                     else
                        docPage
                    )
                    section
                )

        name :: language :: docPage :: _ ->
            Just
                (NameLanguagePage name
                    language
                    (if docPage == "" then
                        "/"

                     else
                        docPage
                    )
                )

        name :: language :: _ ->
            Just (NameLanguage name language)

        name :: _ ->
            Just (Name name)

        _ ->
            Nothing



-- INIT


type alias PageID =
    { project : String
    , language : String
    , page : String
    }


type alias Model =
    { search : Search.Model
    , currentPageID : Maybe PageID
    , page : Maybe (Result Http.Error APISchema.Page)
    , inViewSection : String
    }


init : Maybe ProjectURI -> Maybe String -> ( Model, Effect Msg )
init projectURI maybeID =
    case projectURI of
        Just uri ->
            let
                projectName =
                    projectURIName uri

                maybePageID =
                    projectURIPageID uri

                ( searchModel, searchCmd ) =
                    Search.init (Just projectName)
            in
            ( { search = searchModel
              , currentPageID = Nothing
              , page = Nothing
              , inViewSection = ""
              }
            , Effect.batch
                [ Effect.fromShared (Shared.GetProject projectName)
                , case maybePageID of
                    Just pageID ->
                        Effect.fromCmd (fetchPage pageID)

                    Nothing ->
                        Effect.none
                , case maybeID of
                    Just id ->
                        Effect.fromCmd (scrollIntoViewHack id)

                    Nothing ->
                        Effect.none
                , Effect.fromCmd (Cmd.map (\v -> SearchMsg v) searchCmd)
                ]
            )

        Nothing ->
            let
                ( searchModel, searchCmd ) =
                    Search.init Nothing
            in
            ( { search = searchModel
              , currentPageID = Nothing
              , page = Nothing
              , inViewSection = ""
              }
            , Effect.fromCmd (Cmd.map (\v -> SearchMsg v) searchCmd)
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


projectURIPageID : ProjectURI -> Maybe PageID
projectURIPageID projectURI =
    case projectURI of
        Name _ ->
            Nothing

        NameLanguage _ _ ->
            Nothing

        NameLanguagePage project language pagePath ->
            Just { project = project, language = language, page = pagePath }

        NameLanguagePageSection project language pagePath _ ->
            Just { project = project, language = language, page = pagePath }



-- UPDATE


type Msg
    = NoOp
    | SearchMsg Search.Msg
    | GotPage (Result Http.Error APISchema.Page)
    | ObservePage
    | OnObserved (Result Json.Decode.Error (List Ports.ObserveEvent))


sectionIDs : Schema.Sections -> List String
sectionIDs sections =
    let
        list =
            case sections of
                Schema.Sections v ->
                    v
    in
    List.concatMap
        (\section ->
            List.concat
                [ [ section.id ]
                , if section.detail /= "" then
                    [ String.concat [ section.id, "-content" ] ]

                  else
                    []
                , sectionIDs section.children
                ]
        )
        list


update : Maybe ProjectURI -> Request.With Params -> Msg -> Model -> ( Model, Effect Msg )
update projectURI req msg model =
    case msg of
        NoOp ->
            ( model, Effect.none )

        SearchMsg searchMsg ->
            let
                ( searchModel, searchCmd ) =
                    Search.update searchMsg model.search
            in
            ( { model | search = searchModel }
            , Effect.fromCmd (Cmd.map (\v -> SearchMsg v) searchCmd)
            )

        GotPage result ->
            ( { model | page = Just result }
              -- HACK: having a task here send ObservePage ensures that the view function has ran,
              -- but it's not sufficient to ensure the elements are actually in the DOM yet and so
              -- when update with ObservePage runs, they might not be there. So we use a small delay
            , Effect.fromCmd (Process.sleep 500 |> Task.perform (\_ -> ObservePage))
            )

        ObservePage ->
            case model.page of
                Just response ->
                    case response of
                        Ok docPage ->
                            let
                                observeCmds =
                                    List.map
                                        (\id -> Ports.observeElementID id)
                                        (sectionIDs docPage.sections)
                            in
                            ( model, Effect.fromCmd (Cmd.batch observeCmds) )

                        Err _ ->
                            ( model, Effect.none )

                Nothing ->
                    ( model, Effect.none )

        OnObserved result ->
            case result of
                Ok events ->
                    let
                        newInViewSections =
                            Dict.fromList
                                (List.filterMap
                                    (\v ->
                                        if v.isIntersecting then
                                            Just ( v.targetID, v )

                                        else
                                            Nothing
                                    )
                                    events
                                )

                        byDistanceToCenter =
                            List.sortBy .distanceToCenter
                                (List.map
                                    (\( _, v ) -> v)
                                    (Dict.toList newInViewSections)
                                )

                        inViewSection =
                            Maybe.withDefault model.inViewSection
                                (List.head byDistanceToCenter
                                    |> Maybe.andThen (\v -> Just (trimSuffix v.targetID "-content"))
                                )
                    in
                    ( { model
                        | inViewSection = inViewSection
                      }
                    , let
                        nlp =
                            projectURI
                                |> Maybe.andThen
                                    (\uri ->
                                        case uri of
                                            Name _ ->
                                                Nothing

                                            NameLanguage _ _ ->
                                                Nothing

                                            NameLanguagePage projectName language path ->
                                                Just ( projectName, language, path )

                                            NameLanguagePageSection projectName language path _ ->
                                                Just ( projectName, language, path )
                                    )
                      in
                      case nlp of
                        Just ( projectName, language, path ) ->
                            Effect.fromCmd
                                (Browser.Navigation.replaceUrl
                                    req.key
                                    (Url.Builder.absolute
                                        [ projectName, "-", language, "-", path ]
                                        [ Url.Builder.string "id" inViewSection ]
                                    )
                                )

                        Nothing ->
                            Effect.none
                    )

                Err _ ->
                    ( model, Effect.none )


trimSuffix : String -> String -> String
trimSuffix str suffix =
    if String.endsWith suffix str then
        String.dropRight (String.length suffix) str

    else
        str


fetchPage : PageID -> Cmd Msg
fetchPage pageID =
    Http.get
        { url =
            Url.Builder.absolute [ "api", "get-page" ]
                [ Url.Builder.string "project" pageID.project
                , Url.Builder.string "language" pageID.language
                , Url.Builder.string "page" pageID.page
                ]
        , expect = Http.expectJson GotPage APISchema.pageDecoder
        }



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions _ =
    Ports.onObserved (\events -> OnObserved (Json.Decode.decodeValue Ports.observeEventsDecoder events))



-- VIEW


view : Shared.Model -> Maybe ProjectURI -> Model -> View Msg
view shared projectURI model =
    { title = "doctree"
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
                                        viewName model projectIndexes name

                            Nothing ->
                                E.layout Style.layout (E.text "loading..")

                    Err err ->
                        E.layout Style.layout (E.text (httpErrorToString err))

            Nothing ->
                if shared.flags.cloudMode then
                    E.layout Style.layout (E.text "loading.. (repository may be cloning/indexing which can take up to 120s)")

                else
                    E.layout Style.layout (E.text "loading..")
        ]
    }


viewName : Model -> APISchema.ProjectIndexes -> String -> Html Msg
viewName model projectIndexes projectName =
    E.layout (List.concat [ Style.layout, [ E.width E.fill ] ])
        (E.column [ E.centerX, E.width (E.fill |> E.maximum 700), E.paddingXY 0 64 ]
            [ E.row []
                [ E.link [] { url = "/", label = logo }
                , E.el [ Region.heading 1, Font.size 24 ] (E.text (String.concat [ " / ", projectName ]))
                ]
            , Style.h3 [ E.paddingEach { top = 32, right = 0, bottom = 32, left = 0 } ]
                (E.text (String.concat [ "# Search ", Search.shortProjectName projectName ]))
            , E.map (\v -> SearchMsg v) Search.searchInput
            , if model.search.query /= "" then
                Element.Lazy.lazy
                    (\results -> E.map (\v -> SearchMsg v) (Search.searchResults results))
                    model.search.results

              else
                E.column [ E.alignLeft ]
                    [ Style.h3 [ E.paddingEach { top = 32, right = 0, bottom = 32, left = 0 } ]
                        (E.text "# Browse docs by language")
                    , E.row []
                        (List.map
                            (\language ->
                                E.link
                                    [ Font.underline
                                    , E.paddingXY 16 16
                                    , Border.color (E.rgb255 210 210 210)
                                    , Border.widthEach { top = 6, left = 6, bottom = 6, right = 6 }
                                    ]
                                    { url =
                                        String.concat
                                            [ Url.Builder.absolute [ projectName, "-", language ] [] ]
                                    , label = E.text language
                                    }
                            )
                            (Dict.keys projectIndexes)
                        )
                    ]
            ]
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
    case model.page of
        Just response ->
            case response of
                Ok docPage ->
                    let
                        subpages =
                            case docPage.subpages of
                                Schema.Pages v ->
                                    v
                    in
                    E.layout (List.concat [ Style.layout, [ E.width E.fill, E.height E.fill ] ])
                        (E.row
                            [ E.width E.fill
                            , E.height E.fill
                            ]
                            [ E.el
                                [ -- TODO: Follow the advice at https://package.elm-lang.org/packages/mdgriffith/elm-ui/latest/Element#responsiveness
                                  -- and remove this width hack
                                  E.width (E.px 1)
                                , E.htmlAttribute (Html.Attributes.style "width" "calc(25%)")
                                , E.height E.fill
                                , Background.color (E.rgb 0.95 0.95 0.95)
                                ]
                                (Element.Lazy.lazy
                                    (\( v1, v2 ) -> sidebar v1 v2)
                                    ( model.inViewSection, docPage )
                                )
                            , E.row
                                [ E.width E.fill
                                , E.height E.fill
                                , E.centerX
                                , E.scrollbarY
                                , E.paddingEach { top = 0, right = 0, bottom = 0, left = 48 }
                                ]
                                [ E.column
                                    [ E.width (E.fill |> E.maximum 1000)
                                    , E.height E.fill
                                    ]
                                    [ E.wrappedRow [ E.paddingXY 0 32 ]
                                        [ E.link [ Region.heading 1, Font.size 20 ] { url = Url.Builder.absolute [ projectName ] [], label = E.text projectName }
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
                                    , Element.Lazy.lazy
                                        (\v1 -> renderSections v1)
                                        docPage.sections
                                    ]
                                ]
                            ]
                        )

                Err err ->
                    E.layout Style.layout (E.text (httpErrorToString err))

        Nothing ->
            E.layout Style.layout (E.text "loading..")


renderSections : Schema.Sections -> E.Element Msg
renderSections sections =
    let
        list =
            case sections of
                Schema.Sections v ->
                    v
    in
    E.column [ E.paddingXY 0 16 ]
        (List.map (\section -> renderSection section) list)


renderSection : Schema.Section -> E.Element Msg
renderSection section =
    E.column [ E.width E.fill ]
        [ E.column [ E.width E.fill ]
            [ if section.category then
                Style.h2 [ E.paddingXY 0 8, E.htmlAttribute (Html.Attributes.id section.id) ]
                    (E.text (String.concat [ "# ", section.label ]))

              else
                Style.h3 [ E.paddingXY 0 8, E.htmlAttribute (Html.Attributes.id section.id) ]
                    (E.text
                        (String.concat [ "# ", section.label ])
                    )
            , if section.detail == "" then
                E.none

              else
                E.el
                    [ E.width E.fill
                    , Border.color (E.rgb255 210 210 210)
                    , Border.widthEach { top = 0, left = 6, bottom = 0, right = 0 }
                    , E.paddingXY 16 16
                    , E.htmlAttribute (Html.Attributes.id (String.concat [ section.id, "-content" ]))
                    ]
                    (Markdown.render section.detail)
            ]
        , E.el [ E.width E.fill ] (renderSections section.children)
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


sidebar : String -> APISchema.Page -> E.Element msg
sidebar inViewSection docPage =
    E.column
        [ E.alignRight
        , E.width (E.px 350)
        , E.height E.fill
        , E.scrollbarY
        ]
        [ E.link [ E.centerX, E.paddingXY 0 16 ] { url = "/", label = logo }
        , E.el
            [ E.height E.fill
            , E.width E.fill
            , E.paddingEach { top = 0, right = 64, bottom = 128, left = 16 }
            ]
            (sidebarSections inViewSection 0 docPage.sections)
        ]


sidebarSections : String -> Int -> Schema.Sections -> E.Element msg
sidebarSections inViewSection depth sections =
    let
        list =
            case sections of
                Schema.Sections v ->
                    v
    in
    E.column []
        (List.map
            (\section ->
                E.column [ E.paddingEach { top = 0, right = 0, bottom = 0, left = 32 * depth } ]
                    [ if section.category then
                        Style.h4 [ E.paddingXY 0 16 ] (E.text section.label)

                      else
                        E.link
                            [ Font.underline
                            , E.paddingXY 0 8
                            , if section.id == inViewSection then
                                Font.bold

                              else
                                Font.medium
                            ]
                            { url = "#"
                            , label =
                                E.text
                                    (if section.id == inViewSection then
                                        String.concat [ "* ", section.shortLabel ]

                                     else
                                        section.shortLabel
                                    )
                            }
                    , sidebarSections inViewSection (depth + 1) section.children
                    ]
            )
            list
        )
