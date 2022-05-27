module Main exposing (Model, Msg(..), init, main, subscriptions, update, view)

import Browser
import Browser.Navigation as Nav
import Home
import Html exposing (..)
import Html.Attributes exposing (..)
import Http
import Json.Decode
import Route exposing (Route, toRoute)
import Search
import Url


main : Program () Model Msg
main =
    Browser.application
        { init = init
        , view = view
        , update = update
        , subscriptions = subscriptions
        , onUrlChange = UrlChanged
        , onUrlRequest = LinkClicked
        }


type alias Model =
    { key : Nav.Key
    , url : Url.Url
    , route : Route
    , projectList : Maybe (Result Http.Error (List String))
    , search : Search.Model
    }


init : () -> Url.Url -> Nav.Key -> ( Model, Cmd Msg )
init _ url key =
    let
        ( searchModel, searchCmd ) =
            Search.init Nothing

        route =
            toRoute (Url.toString url)
    in
    ( { key = key
      , url = url
      , route = route
      , projectList = Nothing
      , search = searchModel
      }
    , Cmd.batch
        [ case route of
            Route.Home _ ->
                fetchProjectList

            _ ->
                Cmd.none
        , Cmd.map (\v -> SearchMsg v) searchCmd
        ]
    )


type Msg
    = LinkClicked Browser.UrlRequest
    | UrlChanged Url.Url
    | GotProjectList (Result Http.Error (List String))
    | SearchMsg Search.Msg


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        LinkClicked urlRequest ->
            case urlRequest of
                Browser.Internal url ->
                    ( model, Nav.pushUrl model.key (Url.toString url) )

                Browser.External href ->
                    ( model, Nav.load href )

        UrlChanged url ->
            ( { model | url = url }
            , Cmd.none
            )

        GotProjectList projectList ->
            ( { model | projectList = Just projectList }, Cmd.none )

        SearchMsg searchMsg ->
            let
                ( searchModel, searchCmd ) =
                    Search.update searchMsg model.search
            in
            ( { model | search = searchModel }
            , Cmd.map (\v -> SearchMsg v) searchCmd
            )


{-| TODO: move to Util package
-}
fetchProjectList : Cmd Msg
fetchProjectList =
    Http.get
        { url = "/api/list"
        , expect = Http.expectJson GotProjectList (Json.Decode.list Json.Decode.string)
        }


subscriptions : Model -> Sub Msg
subscriptions _ =
    Sub.none


view : Model -> Browser.Document Msg
view model =
    case model.route of
        -- TODO: use search query param
        Route.Home _ ->
            let
                page =
                    Home.view True { projectList = model.projectList, search = model.search }
            in
            { title = page.title
            , body =
                List.map
                    (\v ->
                        Html.map
                            (\msg ->
                                case msg of
                                    Home.SearchMsg m ->
                                        SearchMsg m
                            )
                            v
                    )
                    page.body
            }

        _ ->
            { title = "doctree"
            , body =
                [ text "TODO: "
                , b [] [ text (Route.toString model.route) ]
                ]
            }
