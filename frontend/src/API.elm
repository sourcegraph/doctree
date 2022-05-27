module API exposing (..)

import APISchema
import Http
import Json.Decode
import Url.Builder
import Util


fetchProjectList : (Result Http.Error (List String) -> msg) -> Cmd msg
fetchProjectList msg =
    Http.get
        { url = "/api/list"
        , expect = Http.expectJson msg (Json.Decode.list Json.Decode.string)
        }


fetchProject : (Result Http.Error APISchema.ProjectIndexes -> msg) -> String -> Cmd msg
fetchProject msg projectName =
    Http.get
        { url = Url.Builder.absolute [ "api", "get" ] [ Url.Builder.string "name" projectName ]
        , expect = Http.expectJson msg APISchema.projectIndexesDecoder
        }


fetchSearchResults :
    (Result Http.Error APISchema.SearchResults -> msg)
    -> String
    -> Bool
    -> Maybe String
    -> Cmd msg
fetchSearchResults msg query intent projectName =
    Http.get
        { url =
            Url.Builder.absolute [ "api", "search" ]
                [ Url.Builder.string "query" query
                , Url.Builder.string "autocomplete" (Util.boolToString (intent == False))
                , Url.Builder.string "project" (Maybe.withDefault "" projectName)
                ]
        , expect = Http.expectJson msg APISchema.searchResultsDecoder
        }
