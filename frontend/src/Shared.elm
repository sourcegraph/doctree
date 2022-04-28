module Shared exposing
    ( Flags
    , Model
    , Msg(..)
    , init
    , subscriptions
    , update
    )

import Http
import Json.Decode as Json
import Request exposing (Request)
import Schema
import Url.Builder


type alias Flags =
    Json.Value


type alias Model =
    { currentProjectName : Maybe String
    , projectIndexes : Maybe (Result Http.Error Schema.ProjectIndexes)
    }


type Msg
    = GetProject String
    | GotProject (Result Http.Error Schema.ProjectIndexes)


init : Request -> Flags -> ( Model, Cmd Msg )
init _ _ =
    ( { currentProjectName = Nothing
      , projectIndexes = Nothing
      }
    , Cmd.none
    )


maybeEquals : a -> a -> Maybe a
maybeEquals v1 v2 =
    if v1 == v2 then
        Just v1

    else
        Nothing


update : Request -> Msg -> Model -> ( Model, Cmd Msg )
update _ msg model =
    case msg of
        GetProject projectName ->
            Maybe.withDefault
                -- No project loaded yet, request it.
                ( { model | currentProjectName = Just projectName }
                , Http.get
                    { url = Url.Builder.absolute [ "api", "get" ] [ Url.Builder.string "name" projectName ]
                    , expect = Http.expectJson GotProject Schema.projectIndexesDecoder
                    }
                )
                -- Loaded already
                (model.currentProjectName
                    |> Maybe.andThen (maybeEquals projectName)
                    |> Maybe.andThen (\_ -> model.projectIndexes)
                    |> Maybe.map (\_ -> ( model, Cmd.none ))
                )

        GotProject result ->
            ( { model | projectIndexes = Just result }, Cmd.none )


subscriptions : Request -> Model -> Sub Msg
subscriptions _ _ =
    Sub.none
