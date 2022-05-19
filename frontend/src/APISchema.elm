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


type alias SearchResults =
    List SearchResult


searchResultsDecoder : Decoder SearchResults
searchResultsDecoder =
    Decode.list searchResultDecoder


searchResultDecoder : Decoder SearchResult
searchResultDecoder =
    Decode.succeed SearchResult
        |> Pipeline.required "language" Decode.string
        |> Pipeline.required "projectName" Decode.string
        |> Pipeline.required "searchKey" Decode.string
        |> Pipeline.required "path" Decode.string
        |> Pipeline.required "id" Decode.string
        |> Pipeline.required "score" Decode.float


type alias SearchResult =
    { language : String
    , projectName : String
    , searchKey : String
    , path : String
    , id : String
    , score : Float
    }
