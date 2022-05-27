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
    | ProjectLanguagePage ProjectName Language PagePath (Maybe SearchQuery)
    | ProjectLanguagePageSection ProjectName Language PagePath SectionID (Maybe SearchQuery)
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
            Url.Builder.absolute [ "/" ] (searchQuerytoParams searchQuery)

        Project projectName searchQuery ->
            Url.Builder.absolute [ projectName ] (searchQuerytoParams searchQuery)

        ProjectLanguage projectName language searchQuery ->
            Url.Builder.absolute [ projectName, "-", language ] (searchQuerytoParams searchQuery)

        ProjectLanguagePage projectName language pagePath searchQuery ->
            Url.Builder.absolute [ projectName, "-", language, "-", pagePath ]
                (searchQuerytoParams searchQuery)

        ProjectLanguagePageSection projectName language pagePath sectionID searchQuery ->
            Url.Builder.absolute [ projectName, "-", language, "-", pagePath ]
                (List.concat
                    [ [ Url.Builder.string "id" sectionID ]
                    , searchQuerytoParams searchQuery
                    ]
                )

        NotFound ->
            Url.Builder.absolute [ "/" ] []


searchQuerytoParams : Maybe SearchQuery -> List QueryParameter
searchQuerytoParams searchQuery =
    case searchQuery of
        Just q ->
            [ Url.Builder.string "q" q ]

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
                <?> Query.string "q"
            )
        , map
            (\projectName language pagePath sectionID searchQuery ->
                case sectionID of
                    Just id ->
                        ProjectLanguagePageSection projectName language pagePath id searchQuery

                    Nothing ->
                        ProjectLanguagePage projectName language pagePath searchQuery
            )
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
