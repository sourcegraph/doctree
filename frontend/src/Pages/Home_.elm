module Pages.Home_ exposing (Model, Msg, page)

import Element as E
import Element.Font as Font
import Gen.Params.Home_ exposing (Params)
import Http
import Json.Decode as D
import Page
import Request
import Shared
import Style
import Util exposing (httpErrorToString)
import View exposing (View)


page : Shared.Model -> Request.With Params -> Page.With Model Msg
page shared req =
    Page.element
        { init = init
        , update = update
        , view = view
        , subscriptions = subscriptions
        }



-- INIT


type alias Model =
    { list : Maybe (Result Http.Error (List String)) }


init : ( Model, Cmd Msg )
init =
    ( { list = Nothing }, fetchList )



-- UPDATE


type Msg
    = GotList (Result Http.Error (List String))


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        GotList list ->
            ( { model | list = Just list }, Cmd.none )


fetchList : Cmd Msg
fetchList =
    Http.get
        { url = "/api/list"
        , expect = Http.expectJson GotList (D.list D.string)
        }



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none



-- VIEW


view : Model -> View Msg
view model =
    { title = "doctree"
    , body =
        [ E.layout (List.concat [ Style.layout, [ E.width E.fill ] ])
            (case model.list of
                Just response ->
                    case response of
                        Ok list ->
                            E.column [ E.centerX, E.paddingXY 0 64 ]
                                [ logo
                                , projectsList list
                                ]

                        Err err ->
                            E.text (httpErrorToString err)

                Nothing ->
                    E.text "loading.."
            )
        ]
    }


logo =
    E.row [ E.centerX ]
        [ E.image
            [ E.width (E.px 120)
            , E.paddingEach { top = 0, right = 140, bottom = 0, left = 0 }
            ]
            { src = "/mascot.svg", description = "cute computer / doctree mascot" }
        , E.column []
            [ E.el [ Font.size 64, Font.bold ] (E.text "doctree")
            , E.el [ Font.semiBold ] (E.text "documentation for every language")
            ]
        ]


projectsList list =
    E.column [ E.centerX ]
        [ Style.h2 [ E.paddingXY 0 32 ] (E.text "# Your projects")
        , E.column []
            (List.map
                (\projectName ->
                    E.link [ E.paddingXY 0 4 ]
                        { url = projectName
                        , label =
                            E.row []
                                [ E.text "• "
                                , E.el [ Font.underline ] (E.text projectName)
                                ]
                        }
                )
                list
            )
        , Style.h2 [ E.paddingXY 0 32 ] (E.text "# Index a project")
        , E.el [ Font.size 16 ] (E.text "$ doctree index -project='foobar' .")
        ]
