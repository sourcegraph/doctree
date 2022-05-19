module Shared exposing
    ( Flags
    , Model
    , Msg(..)
    , init
    , subscriptions
    , update
    )

import APISchema
import Http
import Json.Decode as Decode exposing (Decoder)
import Json.Decode.Pipeline as Pipeline
import Request exposing (Request)
import Url.Builder


type alias Flags =
    Decode.Value


type alias Model =
    { flags : DecodedFlags
    , currentProjectName : Maybe String
    , projectIndexes : Maybe (Result Http.Error APISchema.ProjectIndexes)
    }


type Msg
    = GetProject String
    | GotProject (Result Http.Error APISchema.ProjectIndexes)


flagsDecoder : Decoder DecodedFlags
flagsDecoder =
    Decode.succeed DecodedFlags
        |> Pipeline.required "cloudMode" Decode.bool


type alias DecodedFlags =
    { cloudMode : Bool
    }


init : Request -> Flags -> ( Model, Cmd Msg )
init _ flags =
    ( { currentProjectName = Nothing
      , projectIndexes = Nothing
      , flags =
            Decode.decodeValue flagsDecoder flags
                |> Result.withDefault { cloudMode = False }
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
                    { url = Url.Builder.absolute [ "api", "get-index" ] [ Url.Builder.string "name" projectName ]
                    , expect = Http.expectJson GotProject APISchema.projectIndexesDecoder
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
