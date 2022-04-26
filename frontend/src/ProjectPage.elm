module ProjectPage exposing (Model, Msg, init, page, subscriptions, update, view)

import Gen.Params.NotFound exposing (Params)
import Page
import Request exposing (Request)
import Shared
import Url
import View exposing (View)


page : Shared.Model -> Request.With Params -> Page.With Model Msg
page shared req =
    Page.element
        { init = init
        , update = update
        , view = view req
        , subscriptions = subscriptions
        }



-- INIT


type alias Model =
    {}


init : ( Model, Cmd Msg )
init =
    ( {}, Cmd.none )



-- UPDATE


type Msg
    = ReplaceMe


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        ReplaceMe ->
            ( model, Cmd.none )



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none



-- VIEW


view : Request -> Model -> View Msg
view req model =
    View.placeholder (String.dropLeft (String.length "/") req.url.path)
