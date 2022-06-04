module Flags exposing (Decoded, Flags, decode)

import Json.Decode as Decode
import Json.Decode.Pipeline as Pipeline


type alias Flags =
    Decode.Value


decode : Flags -> Decoded
decode flags =
    Decode.decodeValue decoder flags
        |> Result.withDefault { cloudMode = False }


decoder : Decode.Decoder Decoded
decoder =
    Decode.succeed Decoded
        |> Pipeline.required "cloudMode" Decode.bool


type alias Decoded =
    { cloudMode : Bool
    }
