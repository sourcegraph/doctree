module API exposing (..)

import APISchema
import Http
import Json.Decode
import Url.Builder


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
