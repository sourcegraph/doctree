module Pages.Home_ exposing (Model, Msg, page)

import Element as E
import Gen.Params.Home_ exposing (Params)
import Http
import Json.Decode as D
import Page
import Request
import Shared
import View exposing (View)


page : Shared.Model -> Request.With Params -> Page.With Model Msg
page shared req =
    Page.element
        { init = init
        , update = update
        , view = view
        , subscriptions = subscriptions
        }



-- INIT


type alias Model =
    { list : Maybe (Result Http.Error (List String)) }


init : ( Model, Cmd Msg )
init =
    ( { list = Nothing }, fetchList )



-- UPDATE


type Msg
    = GotList (Result Http.Error (List String))


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        GotList list ->
            ( { model | list = Just list }, Cmd.none )


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


view : Model -> View Msg
view model =
    { title = "doctree"
    , body =
        [ E.layout []
            (case model.list of
                Just response ->
                    case response of
                        Ok list ->
                            E.column [] (List.map (\projectName -> E.link [] { url = projectName, label = E.text projectName }) list)

                        Err err ->
                            E.text (Debug.toString err)

                Nothing ->
                    E.text "loading.."
            )
        ]
    }
