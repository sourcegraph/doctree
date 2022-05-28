module Project exposing (..)

import API
import APISchema
import Browser
import Browser.Dom
import Browser.Navigation
import Dict exposing (keys)
import Element as E
import Element.Background as Background
import Element.Border as Border
import Element.Font as Font
import Element.Input exposing (search)
import Element.Lazy
import Element.Region as Region
import Html exposing (section)
import Html.Attributes
import Http
import Json.Decode
import Markdown
import Ports
import Process
import Route as Router exposing (Language, PagePath, ProjectName, SearchQuery, SectionID)
import Schema
import Search
import Style
import Task
import Url.Builder
import Util exposing (httpErrorToString)


type alias Model =
    { pageID : Maybe API.PageID
    , page : Maybe (Result Http.Error APISchema.Page)
    , inViewSection : String
    }


type Route
    = Project ProjectName (Maybe SearchQuery)
    | ProjectLanguage ProjectName Language (Maybe SearchQuery)
    | ProjectLanguagePage ProjectName Language PagePath (Maybe SectionID) (Maybe SearchQuery)


fromRoute : Router.Route -> Maybe Route
fromRoute route =
    case route of
        Router.Project a b ->
            Just (Project a b)

        Router.ProjectLanguage a b c ->
            Just (ProjectLanguage a b c)

        Router.ProjectLanguagePage a b c d e ->
            Just (ProjectLanguagePage a b c d e)

        _ ->
            Nothing


init : Route -> ( Model, Cmd Msg )
init route =
    case route of
        -- TODO: use searchQuery parameter
        Project projectName _ ->
            ( { pageID = Nothing, page = Nothing, inViewSection = "" }
            , Task.succeed () |> Task.perform (\_ -> GetProject projectName)
            )

        -- TODO: use searchQuery parameter
        ProjectLanguage projectName _ _ ->
            ( { pageID = Nothing, page = Nothing, inViewSection = "" }
            , Task.succeed () |> Task.perform (\_ -> GetProject projectName)
            )

        -- TODO: use searchQuery parameter
        ProjectLanguagePage projectName language pagePath sectionID _ ->
            let
                pageID =
                    { projectName = projectName
                    , language = language
                    , pagePath = pagePath
                    }
            in
            ( { pageID = Just pageID, page = Nothing, inViewSection = "" }
            , Cmd.batch
                [ API.fetchPage GotPage pageID
                , case sectionID of
                    Just id ->
                        attemptScrollIntoView id

                    Nothing ->
                        Cmd.none
                ]
            )


attemptScrollIntoView : String -> Cmd Msg
attemptScrollIntoView id =
    scrollIntoView id
        |> Task.attempt
            (\result ->
                case result of
                    Ok _ ->
                        NoOp

                    Err _ ->
                        ScrollIntoViewLater id
            )


scrollIntoView : String -> Task.Task Browser.Dom.Error ()
scrollIntoView id =
    Browser.Dom.getViewportOf "content-area"
        |> Task.andThen
            (\contentArea ->
                Task.map
                    (\info -> contentArea.viewport.y + info.element.y)
                    (Browser.Dom.getElement id)
            )
        |> Task.andThen (\y -> Browser.Dom.setViewportOf "content-area" 0 y)



-- TODO: cleanup this distinction between Msg and UpdateMsg


type Msg
    = NoOp
    | SearchMsg Search.Msg
    | GetProject String
    | GotPage (Result Http.Error APISchema.Page)
    | ObservePage
    | OnObserved (Result Json.Decode.Error (List Ports.ObserveEvent))
    | ScrollIntoViewLater String
    | ReplaceUrlSilently String


type UpdateMsg
    = UpdateNoOp
    | UpdateGotPage (Result Http.Error APISchema.Page)
    | UpdateObservePage
    | UpdateOnObserved (Result Json.Decode.Error (List Ports.ObserveEvent))
    | UpdateScrollIntoViewLater String
    | NavigateToSectionID (Maybe SectionID)


update : Browser.Navigation.Key -> UpdateMsg -> Model -> ( Model, Cmd Msg )
update key msg model =
    case msg of
        UpdateNoOp ->
            ( model, Cmd.none )

        UpdateGotPage result ->
            ( { model | page = Just result }
              -- HACK: having a task here send ObservePage ensures that the view function has ran,
              -- but it's not sufficient to ensure the elements are actually in the DOM yet and so
              -- when update with ObservePage runs, they might not be there. So we use a small delay
              --
              -- TODO: use proper error handling + delay to deal with this.
            , Process.sleep 500 |> Task.perform (\_ -> ObservePage)
            )

        UpdateObservePage ->
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
                            ( model, Cmd.batch observeCmds )

                        Err _ ->
                            ( model, Cmd.none )

                Nothing ->
                    ( model, Cmd.none )

        UpdateOnObserved result ->
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
                                    |> Maybe.andThen (\v -> Just (Util.trimSuffix v.targetID "-content"))
                                )
                    in
                    ( { model
                        | inViewSection = inViewSection
                      }
                    , case model.pageID of
                        Just p ->
                            Task.succeed ()
                                |> Task.perform
                                    (\_ ->
                                        ReplaceUrlSilently
                                            (Router.toString
                                                -- TODO: Retain search query string
                                                (Router.ProjectLanguagePage p.projectName
                                                    p.language
                                                    p.pagePath
                                                    (Just inViewSection)
                                                    Nothing
                                                )
                                            )
                                    )

                        Nothing ->
                            Cmd.none
                    )

                Err _ ->
                    ( model, Cmd.none )

        UpdateScrollIntoViewLater id ->
            ( model
            , Process.sleep 16
                |> Task.andThen (\_ -> scrollIntoView id)
                |> Task.attempt
                    (\result ->
                        case result of
                            Ok _ ->
                                NoOp

                            Err _ ->
                                ScrollIntoViewLater id
                    )
            )

        NavigateToSectionID sectionID ->
            case sectionID of
                Just id ->
                    ( model, attemptScrollIntoView id )

                Nothing ->
                    ( model, Cmd.none )


subscriptions : Model -> Sub Msg
subscriptions _ =
    Ports.onObserved
        (\events ->
            OnObserved
                (Json.Decode.decodeValue Ports.observeEventsDecoder events)
        )


viewProject :
    Search.Model
    -> Maybe (Result Http.Error APISchema.ProjectIndexes)
    -> ProjectName
    -> Maybe SearchQuery
    -> Bool
    -> Model
    -> Browser.Document Msg
viewProject search projectIndexesRequest projectName _ cloudMode _ =
    { title = "doctree"
    , body =
        [ case projectIndexesRequest of
            Just response ->
                case response of
                    Ok projectIndexes ->
                        E.layout (List.concat [ Style.layout, [ E.width E.fill ] ])
                            (E.column [ E.centerX, E.width (E.fill |> E.maximum 700), E.paddingXY 0 64 ]
                                [ E.row []
                                    [ E.link [] { url = "/", label = logo }
                                    , E.el [ Region.heading 1, Font.size 24 ] (E.text (String.concat [ " / ", projectName ]))
                                    ]
                                , Style.h3 [ E.paddingEach { top = 32, right = 0, bottom = 32, left = 0 } ]
                                    (E.text (String.concat [ "# Search ", Util.shortProjectName projectName ]))
                                , E.map (\v -> SearchMsg v) Search.searchInput
                                , if search.query /= "" then
                                    Element.Lazy.lazy
                                        (\results -> E.map (\v -> SearchMsg v) (Search.searchResults results))
                                        search.results

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

                    Err err ->
                        E.layout Style.layout (E.text (httpErrorToString err))

            Nothing ->
                if cloudMode then
                    E.layout Style.layout (E.text "loading.. (repository may be cloning/indexing which can take up to 120s)")

                else
                    E.layout Style.layout (E.text "loading..")
        ]
    }


viewProjectLanguage :
    Search.Model
    -> Maybe (Result Http.Error APISchema.ProjectIndexes)
    -> ProjectName
    -> Language
    -> Maybe SearchQuery
    -> Bool
    -> Model
    -> Browser.Document Msg
viewProjectLanguage _ projectIndexesRequest projectName language _ cloudMode _ =
    { title = "doctree"
    , body =
        [ case projectIndexesRequest of
            Just response ->
                case response of
                    Ok projectIndexes ->
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

                    Err err ->
                        E.layout Style.layout (E.text (httpErrorToString err))

            Nothing ->
                if cloudMode then
                    E.layout Style.layout (E.text "loading.. (repository may be cloning/indexing which can take up to 120s)")

                else
                    E.layout Style.layout (E.text "loading..")
        ]
    }


viewProjectLanguagePage :
    Search.Model
    -> ProjectName
    -> Language
    -> PagePath
    -> Maybe SectionID
    -> Maybe SearchQuery
    -> Model
    -> Browser.Document Msg
viewProjectLanguagePage _ projectName language pagePath _ searchQuery model =
    { title = "doctree"
    , body =
        [ case model.page of
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
                                        (\v -> sidebar v.projectName v.language v.pagePath v.searchQuery v.inViewSection v.docPage)
                                        { projectName = projectName
                                        , language = language
                                        , pagePath = pagePath
                                        , searchQuery = searchQuery
                                        , inViewSection = model.inViewSection
                                        , docPage = docPage
                                        }
                                    )
                                , E.row
                                    [ E.width E.fill
                                    , E.height E.fill
                                    , E.centerX
                                    , E.scrollbarY
                                    , E.htmlAttribute (Html.Attributes.id "content-area")
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
        ]
    }


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


sidebar :
    ProjectName
    -> Language
    -> PagePath
    -> Maybe SearchQuery
    -> String
    -> APISchema.Page
    -> E.Element msg
sidebar projectName language pagePath searchQuery inViewSection docPage =
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
            (sidebarSections projectName language pagePath searchQuery inViewSection 0 docPage.sections)
        ]


sidebarSections :
    ProjectName
    -> Language
    -> PagePath
    -> Maybe SearchQuery
    -> String
    -> Int
    -> Schema.Sections
    -> E.Element msg
sidebarSections projectName language pagePath searchQuery inViewSection depth sections =
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
                            { url =
                                Router.toString
                                    (Router.ProjectLanguagePage projectName
                                        language
                                        pagePath
                                        (Just section.id)
                                        searchQuery
                                    )
                            , label =
                                E.text
                                    (if section.id == inViewSection then
                                        String.concat [ "* ", section.shortLabel ]

                                     else
                                        section.shortLabel
                                    )
                            }
                    , sidebarSections projectName language pagePath searchQuery inViewSection (depth + 1) section.children
                    ]
            )
            list
        )
