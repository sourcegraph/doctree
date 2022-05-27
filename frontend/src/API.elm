module API exposing (..)

import Http
import Json.Decode


fetchProjectList : (Result Http.Error (List String) -> msg) -> Cmd msg
fetchProjectList msg =
    Http.get
        { url = "/api/list"
        , expect = Http.expectJson msg (Json.Decode.list Json.Decode.string)
        }
