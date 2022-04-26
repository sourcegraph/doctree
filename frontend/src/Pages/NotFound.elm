module Pages.NotFound exposing (Model, Msg, page)

-- elm-spa will invoke our NotFound page for any undefined route. We don't want
-- to register a route for each /github.com/foo/bar, /gitlab.com/foo/bar, /project/path,
-- etc. and so we direct our NotFound handler straight to the ProjectPage. elm-spa
-- doesn't have a nicer way to do this.

import Page
import ProjectPage
import Request exposing (Request)
import Shared


page : Shared.Model -> Request -> Page.With Model Msg
page shared req =
    ProjectPage.page shared req


type alias Model =
    ProjectPage.Model


type alias Msg =
    ProjectPage.Msg
