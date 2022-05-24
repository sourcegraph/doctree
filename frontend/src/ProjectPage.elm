module ProjectPage exposing (Model, Msg, init, page, subscriptions, update, view)

import APISchema
import Browser.Dom
import Dict exposing (keys)
import Effect exposing (Effect)
import Element as E
import Element.Border as Border
import Element.Font as Font
import Element.Lazy
import Element.Region as Region
import Gen.Params.NotFound exposing (Params)
import Html exposing (Html)
import Html.Attributes
import Http
import InView
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
    , inView : InView.State
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

                ( inViewModel, inViewCmds ) =
                    InView.init InViewMsg []
            in
            ( { search = searchModel
              , currentPageID = Nothing
              , page = Nothing
              , inView = inViewModel
              , inViewSection = ""
              }
            , Effect.batch
                [ Effect.fromShared (Shared.GetProject projectName)
                , Effect.fromCmd inViewCmds
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

                ( inViewModel, inViewCmds ) =
                    InView.init InViewMsg []
            in
            ( { search = searchModel
              , currentPageID = Nothing
              , page = Nothing
              , inView = inViewModel
              , inViewSection = ""
              }
            , Effect.batch
                [ Effect.fromCmd (Cmd.map (\v -> SearchMsg v) searchCmd)
                , Effect.fromCmd inViewCmds
                ]
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
    | OnScroll { x : Float, y : Float }
    | InViewMsg InView.Msg


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
                [ [ section.id
                  , String.concat [ section.id, "-content" ]
                  ]
                , sectionIDs section.children
                ]
        )
        list


update : Msg -> Model -> ( Model, Effect Msg )
update msg model =
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
            case result of
                Ok docPage ->
                    let
                        ( inView, inViewCmds ) =
                            InView.addElements InViewMsg (sectionIDs docPage.sections) model.inView
                    in
                    ( { model | page = Just result, inView = inView }, Effect.fromCmd inViewCmds )

                Err _ ->
                    ( { model | page = Just result }, Effect.none )

        OnScroll offset ->
            ( { model
                | inView = InView.updateViewportOffset offset model.inView
                , inViewSection = Maybe.withDefault "" (determineInViewSection model)
              }
            , Effect.none
            )

        InViewMsg inViewMsg ->
            let
                ( inView, inViewCmds ) =
                    InView.update InViewMsg inViewMsg model.inView
            in
            ( { model
                | inView = inView
                , inViewSection = Maybe.withDefault "" (determineInViewSection model)
              }
            , Effect.fromCmd inViewCmds
            )


determineInViewSection : Model -> Maybe String
determineInViewSection model =
    model.page
        |> Maybe.andThen
            (\response ->
                case response of
                    Ok docPage ->
                        let
                            inViewSections =
                                List.map
                                    (\sectionID ->
                                        let
                                            margin =
                                                { top = 100, right = 0, bottom = 100, left = 0 }

                                            contentDist =
                                                Maybe.withDefault 1000000 (isInViewDistance (String.concat [ sectionID, "-content" ]) margin model.inView)

                                            dist =
                                                Maybe.withDefault contentDist (isInViewDistance sectionID margin model.inView)
                                        in
                                        { sectionID = sectionID, dist = dist }
                                    )
                                    (sectionIDs docPage.sections)

                            inViewSection =
                                List.head (List.sortBy .dist inViewSections)
                        in
                        inViewSection |> Maybe.andThen (\section -> Just (trimSuffix section.sectionID "-content"))

                    Err _ ->
                        Nothing
            )


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
subscriptions model =
    Sub.batch
        [ InView.subscriptions InViewMsg model.inView
        , Ports.onScroll OnScroll
        ]



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
                            , renderSections model docPage.sections
                            ]
                        )

                Err err ->
                    E.layout Style.layout (E.text (httpErrorToString err))

        Nothing ->
            E.layout Style.layout (E.text "loading..")


maxWidth =
    E.width (E.fill |> E.maximum 1000)


renderSections : Model -> Schema.Sections -> E.Element Msg
renderSections model sections =
    let
        list =
            case sections of
                Schema.Sections v ->
                    v
    in
    E.column [ maxWidth, E.paddingXY 0 16 ]
        (List.map (\section -> renderSection model section) list)


isInViewDistance : String -> InView.Margin -> InView.State -> Maybe Float
isInViewDistance id margin state =
    let
        calc { viewport } element =
            if
                (viewport.y + margin.top < element.y + element.height)
                    && (viewport.y + viewport.height - margin.bottom > element.y)
                    && (viewport.x + margin.left < element.x + element.width)
                    && (viewport.x + viewport.width - margin.right > element.x)
            then
                Basics.abs ((viewport.y + (viewport.height / 2)) - (element.y + (element.height / 2)))

            else
                1000000
    in
    InView.custom (\a b -> Maybe.map (calc a) b) id state


renderSection : Model -> Schema.Section -> E.Element Msg
renderSection model section =
    E.column [ E.width E.fill ]
        [ E.column [ E.width E.fill ]
            [ if section.category then
                Style.h2 [ E.paddingXY 0 8, E.htmlAttribute (Html.Attributes.id section.id) ]
                    (E.text (String.concat [ "# ", section.label ]))

              else
                Style.h3 [ E.paddingXY 0 8, E.htmlAttribute (Html.Attributes.id section.id) ]
                    (E.text
                        (String.concat
                            [ "# "
                            , section.label
                            , if section.id == model.inViewSection then
                                " *"

                              else
                                ""
                            ]
                        )
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
        , E.el [ E.width E.fill ] (renderSections model section.children)
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
