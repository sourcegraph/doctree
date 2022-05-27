module Route exposing
    ( Language
    , PagePath
    , ProjectName
    , Route
    , SearchQuery
    , SectionID
    , toRoute
    , toString
    )

import Url
import Url.Builder exposing (QueryParameter)
import Url.Parser exposing ((</>), (<?>), Parser, custom, map, oneOf, parse, s, string, top)
import Url.Parser.Query as Query


type alias SearchQuery =
    String


type alias ProjectName =
    String


type alias Language =
    String


type alias PagePath =
    String


type alias SectionID =
    String


type Route
    = Home (Maybe SearchQuery)
    | Project ProjectName (Maybe SearchQuery)
    | ProjectLanguage ProjectName Language (Maybe SearchQuery)
    | ProjectLanguagePage ProjectName Language PagePath (Maybe SectionID) (Maybe SearchQuery)
    | NotFound


toRoute : String -> Route
toRoute string =
    case Url.fromString string of
        Nothing ->
            NotFound

        Just url ->
            Maybe.withDefault NotFound (parse routeParser url)


toString : Route -> String
toString route =
    case route of
        Home searchQuery ->
            Url.Builder.absolute [] (maybeParam "q" searchQuery)

        Project projectName searchQuery ->
            Url.Builder.absolute [ projectName ] (maybeParam "q" searchQuery)

        ProjectLanguage projectName language searchQuery ->
            Url.Builder.absolute [ projectName, "-", language ] (maybeParam "q" searchQuery)

        ProjectLanguagePage projectName language pagePath sectionID searchQuery ->
            Url.Builder.absolute [ projectName, "-", language, "-", pagePath ]
                (List.concat
                    [ maybeParam "id" sectionID
                    , maybeParam "q" searchQuery
                    ]
                )

        NotFound ->
            Url.Builder.absolute [] []


maybeParam : String -> Maybe String -> List QueryParameter
maybeParam name value =
    case value of
        Just v ->
            [ Url.Builder.string name v ]

        Nothing ->
            []


routeParser : Parser (Route -> a) a
routeParser =
    oneOf
        [ map Home (top <?> Query.string "q")
        , map Project (projectNameParser <?> Query.string "q")
        , map ProjectLanguage (projectNameParser </> s "-" </> string <?> Query.string "q")
        , map ProjectLanguagePage
            (projectNameParser
                </> s "-"
                </> string
                </> s "-"
                </> pagePathParser
                <?> Query.string "id"
                <?> Query.string "q"
            )
        ]


projectNameParser : Parser (String -> a) a
projectNameParser =
    oneOf
        [ map (\a b c d e -> String.join "/" [ a, b, c, d, e ]) (notDash </> notDash </> notDash </> notDash </> notDash)
        , map (\a b c d -> String.join "/" [ a, b, c, d ]) (notDash </> notDash </> notDash </> notDash)
        , map (\a b c -> String.join "/" [ a, b, c ]) (notDash </> notDash </> notDash)
        , map (\a b -> String.join "/" [ a, b ]) (notDash </> notDash)
        , notDash
        ]


pagePathParser : Parser (String -> a) a
pagePathParser =
    oneOf
        [ map (\a b c d e f g h i -> String.join "/" [ a, b, c, d, e, f, g, h, i ]) (string </> string </> string </> string </> string </> string </> string </> string </> string)
        , map (\a b c d e f g h -> String.join "/" [ a, b, c, d, e, f, g, h ]) (string </> string </> string </> string </> string </> string </> string </> string)
        , map (\a b c d e f g -> String.join "/" [ a, b, c, d, e, f, g ]) (string </> string </> string </> string </> string </> string </> string)
        , map (\a b c d e f -> String.join "/" [ a, b, c, d, e, f ]) (string </> string </> string </> string </> string </> string)
        , map (\a b c d e -> String.join "/" [ a, b, c, d, e ]) (string </> string </> string </> string </> string)
        , map (\a b c d -> String.join "/" [ a, b, c, d ]) (string </> string </> string </> string)
        , map (\a b c -> String.join "/" [ a, b, c ]) (string </> string </> string)
        , map (\a b -> String.join "/" [ a, b ]) (string </> string)
        , notDash
        ]


notDash : Parser (String -> a) a
notDash =
    custom "NOT_DASH" <|
        \segment ->
            if segment /= "-" then
                Just segment

            else
                Nothing
