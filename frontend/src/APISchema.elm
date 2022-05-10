module APISchema exposing (..)

import Dict exposing (Dict)
import Json.Decode as Decode exposing (Decoder)
import Json.Decode.Pipeline as Pipeline
import Schema



-- decoder for: /api/list


type alias ProjectList =
    List String


projectListDecoder : Decoder ProjectList
projectListDecoder =
    Decode.list Decode.string



-- decoder for: /api/get?name=github.com/sourcegraph/sourcegraph


type alias ProjectIndexes =
    Dict String Schema.Index


projectIndexesDecoder : Decoder ProjectIndexes
projectIndexesDecoder =
    Decode.dict Schema.indexDecoder



-- decoder for: /api/search?query=foobar


searchResultDecoder : Decoder SearchResult
searchResultDecoder =
    Decode.succeed SearchResult
        |> Pipeline.required "path" Decode.string
        |> Pipeline.required "keys" (Decode.list Decode.string)


type alias SearchResult =
    { path : String
    , keys : List String
    }
