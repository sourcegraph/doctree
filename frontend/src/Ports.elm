port module Ports exposing (..)

import Json.Decode as Decode exposing (Decoder)
import Json.Decode.Pipeline as Pipeline


port observeElementID : String -> Cmd msg


port onObserved : (Decode.Value -> msg) -> Sub msg


observeEventsDecoder : Decoder (List ObserveEvent)
observeEventsDecoder =
    Decode.list observeEventDecoder


type alias ObserveEvent =
    { isIntersecting : Bool
    , intersectionRatio : Float
    , distanceToCenter : Float
    , targetID : String
    }


observeEventDecoder : Decoder ObserveEvent
observeEventDecoder =
    Decode.succeed ObserveEvent
        |> Pipeline.required "isIntersecting" Decode.bool
        |> Pipeline.required "intersectionRatio" Decode.float
        |> Pipeline.required "distanceToCenter" Decode.float
        |> Pipeline.required "targetID" Decode.string
