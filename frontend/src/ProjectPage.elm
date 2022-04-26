module ProjectPage exposing (Model, Msg, init, page, subscriptions, update, view)

import Dict
import Element as E
import Gen.Params.NotFound exposing (Params)
import Html exposing (Html)
import Http
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
    Page.element
        { init = init projectURI
        , update = update
        , view = view projectURI
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
    { projectIndexes : Maybe (Result Http.Error Schema.ProjectIndexes) }


init : Maybe ProjectURI -> ( Model, Cmd Msg )
init projectURI =
    ( { projectIndexes = Nothing }
    , case projectURI of
        Just uri ->
            fetchProject uri

        Nothing ->
            Cmd.none
    )


fetchProject : ProjectURI -> Cmd Msg
fetchProject projectURI =
    let
        projectName =
            case projectURI of
                Name v ->
                    v

                NameLanguage v _ ->
                    v

                NameLanguagePage v _ _ ->
                    v

                NameLanguagePageSection v _ _ _ ->
                    v
    in
    Http.get
        { url = Url.Builder.absolute [ "api", "get" ] [ Url.Builder.string "name" projectName ]
        , expect = Http.expectJson GotProject Schema.projectIndexesDecoder
        }



-- UPDATE


type Msg
    = GotProject (Result Http.Error Schema.ProjectIndexes)


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        GotProject projectIndexes ->
            ( { model | projectIndexes = Just projectIndexes }, Cmd.none )



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none



-- VIEW


view : Maybe ProjectURI -> Model -> View Msg
view projectURI model =
    { title = "TODO"
    , body =
        [ case model.projectIndexes of
            Just response ->
                case response of
                    Ok projectIndexes ->
                        case projectURI of
                            Just uri ->
                                case uri of
                                    NameLanguagePageSection name language docPage section ->
                                        -- viewNameLanguagePageSection model name language docPage section
                                        viewName model projectIndexes name

                                    NameLanguagePage name language docPage ->
                                        -- viewNameLanguagePage model name language docPage
                                        viewName model projectIndexes name

                                    NameLanguage name language ->
                                        viewNameLanguage model projectIndexes name language

                                    Name name ->
                                        viewName model projectIndexes name

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
        (E.row []
            (List.map (\language -> E.link [] { url = String.concat [ Url.Builder.absolute [ projectName, "-", language ] [] ], label = E.text language }) (Dict.keys projectIndexes))
        )


viewNameLanguage : Model -> Schema.ProjectIndexes -> String -> String -> Html Msg
viewNameLanguage model projectIndexes projectName language =
    let
        indexLookup =
            Dict.get language projectIndexes
    in
    case indexLookup of
        Just index ->
            E.layout []
                (E.row []
                    [ E.text index.schemaVersion ]
                )

        Nothing ->
            E.layout [] (E.text "language not found")
