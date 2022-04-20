module Pages.Home_ exposing (view)

import Html
import View exposing (View)


view : View msg
view =
    { title = "doctree"
    , body = [ Html.text "Hello, doctree!" ]
    }
