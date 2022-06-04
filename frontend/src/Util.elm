module Util exposing (..)

import Http


httpErrorToString : Http.Error -> String
httpErrorToString error =
    case error of
        Http.BadUrl url ->
            "The URL " ++ url ++ " was invalid"

        Http.Timeout ->
            "Unable to reach the server, try again"

        Http.NetworkError ->
            "Unable to reach the server, check your network connection"

        Http.BadStatus 500 ->
            "The server had a problem, try again later"

        Http.BadStatus 400 ->
            "Verify your information and try again"

        Http.BadStatus _ ->
            "Unknown error"

        Http.BadBody errorMessage ->
            errorMessage


maybeEquals : a -> a -> Maybe a
maybeEquals v1 v2 =
    if v1 == v2 then
        Just v1

    else
        Nothing


shortProjectName : String -> String
shortProjectName name =
    trimPrefix name "github.com/"


trimPrefix : String -> String -> String
trimPrefix str prefix =
    if String.startsWith prefix str then
        String.dropLeft (String.length prefix) str

    else
        str


trimSuffix : String -> String -> String
trimSuffix str suffix =
    if String.endsWith suffix str then
        String.dropRight (String.length suffix) str

    else
        str


boolToString : Bool -> String
boolToString value =
    if value then
        "true"

    else
        "false"
