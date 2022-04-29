module ProjectPage exposing (Model, Msg, init, page, subscriptions, update, view)

import Dict
import Effect exposing (Effect)
import Element as E
import Element.Font as Font
import Element.Region as Region
import Gen.Params.NotFound exposing (Params)
import Html exposing (Html)
import Page
import Request
import Schema
import Shared
import Url.Builder
import View exposing (View)


page : Shared.Model -> Request.With Params -> Page.With Model Msg
page shared req =
    let
        rawProjectURI =
            String.dropLeft (String.length "/") req.url.path

        projectURI =
            parseProjectURI rawProjectURI
    in
    Page.advanced
        { init = init projectURI
        , update = update
        , view = view shared projectURI
        , subscriptions = subscriptions
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


init : Maybe ProjectURI -> ( Model, Effect Msg )
init projectURI =
    ( {}
    , case projectURI of
        Just uri ->
            Effect.fromShared (Shared.GetProject (projectURIName uri))

        Nothing ->
            Effect.none
    )


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
                                        viewNameLanguage model projectIndexes name "go"

                            Nothing ->
                                E.layout [] (E.text "loading..")

                    Err err ->
                        E.layout [] (E.text (Debug.toString err))

            Nothing ->
                E.layout [] (E.text "loading..")
        ]
    }


viewName : Model -> Schema.ProjectIndexes -> String -> Html Msg
viewName model projectIndexes projectName =
    E.layout []
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


viewNameLanguage : Model -> Schema.ProjectIndexes -> String -> String -> Html Msg
viewNameLanguage model projectIndexes projectName language =
    let
        indexLookup =
            Dict.get language projectIndexes
    in
    case indexLookup of
        Just index ->
            E.layout [ E.width E.fill ]
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
                                            Url.Builder.absolute [ projectName, "-", language, "-", docPage.path ] []
                                        , label = E.el [ Font.underline ] (E.text docPage.path)
                                        }
                                )
                                (List.sortBy .path index.library.pages)
                            )
                        , E.column [ E.width (E.fillPortion 1) ]
                            (List.map
                                (\docPage ->
                                    E.link [ E.width (E.fillPortion 1) ]
                                        { url =
                                            Url.Builder.absolute [ projectName, "-", language, "-", docPage.path ] []
                                        , label = E.el [ Font.underline ] (E.text docPage.title)
                                        }
                                )
                                (List.sortBy .path index.library.pages)
                            )
                        ]
                    ]
                )

        Nothing ->
            E.layout [] (E.text "language not found")


viewNameLanguagePage : Model -> Schema.ProjectIndexes -> String -> String -> String -> Html Msg
viewNameLanguagePage model projectIndexes projectName language targetPagePath =
    let
        indexLookup =
            Dict.get language projectIndexes

        pageLookup =
            Maybe.andThen
                (\index ->
                    List.head
                        (List.filter (\docPage -> docPage.path == targetPagePath) index.library.pages)
                )
                indexLookup
    in
    case pageLookup of
        Just docPage ->
            let
                subpages =
                    case docPage.subpages of
                        Schema.Pages v ->
                            v
            in
            E.layout []
                (E.column []
                    [ E.column []
                        [ E.el [ Region.heading 1, Font.size 32 ] (E.text docPage.title)
                        , E.text docPage.detail
                        , if List.length subpages > 0 then
                            E.column []
                                (List.concat
                                    [ [ E.el [ Region.heading 1, Font.size 32 ] (E.text "Subpages") ]
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
            E.layout [] (E.text "page not found")


renderSections : Schema.Sections -> E.Element Msg
renderSections sections =
    let
        list =
            case sections of
                Schema.Sections v ->
                    v
    in
    E.column []
        (List.map (\section -> renderSection section) list)


renderSection : Schema.Section -> E.Element Msg
renderSection section =
    E.column []
        [ E.column [ E.paddingXY 0 16 ]
            [ E.text section.label
            , E.text section.detail
            ]
        , E.el [ E.paddingXY 32 0 ] (renderSections section.children)
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
