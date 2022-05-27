module Markdown exposing (..)

{-| This renders `Html` in an attempt to be as close as possible to
the HTML output in <https://github.github.com/gfm/>.
-}

import Element as E
import Html.Attributes
import Markdown.Parser as Markdown
import Markdown.Renderer


render markdown =
    if False then
        -- DEBUG: Render Markdown as plain text for debugging.
        E.textColumn []
            (List.map
                (\paragraph -> E.paragraph [ E.paddingXY 0 4 ] [ E.text paragraph ])
                (String.split "\n" markdown)
            )

    else
        case
            markdown
                |> Markdown.parse
                |> Result.mapError deadEndsToString
                |> Result.andThen (\ast -> Markdown.Renderer.render Markdown.Renderer.defaultHtmlRenderer ast)
        of
            Ok rendered ->
                E.column [ E.htmlAttribute (Html.Attributes.class "markdown") ] (List.map (\e -> E.html e) rendered)

            Err errors ->
                E.text errors


deadEndsToString deadEnds =
    deadEnds
        |> List.map Markdown.deadEndToString
        |> String.join "\n"
