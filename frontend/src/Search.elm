module Search exposing (..)

import API
import APISchema
import Browser.Dom
import Element as E
import Element.Border as Border
import Element.Font as Font
import Html
import Html.Attributes
import Html.Events
import Http
import Process
import Task
import Url.Builder
import Util exposing (httpErrorToString)


debounceQueryInputMillis : Float
debounceQueryInputMillis =
    20


debounceQueryIntentInputMillis : Float
debounceQueryIntentInputMillis =
    500



-- INIT


type alias Model =
    { debounce : Int
    , debounceIntent : Int
    , query : String
    , projectName : Maybe String
    , results : Maybe (Result Http.Error APISchema.SearchResults)
    }


init : Maybe String -> ( Model, Cmd Msg )
init projectName =
    ( { debounce = 0
      , debounceIntent = 0
      , query = ""
      , projectName = projectName
      , results = Nothing
      }
    , Task.perform
        (\_ -> FocusOn "search-input")
        (Process.sleep 100)
    )



-- UPDATE


type Msg
    = FocusOn String
    | OnSearchInput String
    | OnDebounce
    | OnDebounceIntent
    | RunSearch Bool
    | GotSearchResults (Result Http.Error APISchema.SearchResults)
    | NoOp


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        OnSearchInput query ->
            ( { model
                | query = query
                , debounce = model.debounce + 1
                , debounceIntent = model.debounceIntent + 1
              }
            , Cmd.batch
                [ Task.perform (\_ -> OnDebounce) (Process.sleep debounceQueryInputMillis)
                , Task.perform (\_ -> OnDebounceIntent) (Process.sleep debounceQueryIntentInputMillis)
                ]
            )

        OnDebounce ->
            if model.debounce - 1 == 0 then
                update (RunSearch False) { model | debounce = model.debounce - 1 }

            else
                ( { model | debounce = model.debounce - 1 }, Cmd.none )

        OnDebounceIntent ->
            if model.debounceIntent - 1 == 0 then
                update (RunSearch True) { model | debounceIntent = model.debounceIntent - 1 }

            else
                ( { model | debounceIntent = model.debounceIntent - 1 }, Cmd.none )

        RunSearch intent ->
            ( model, API.fetchSearchResults GotSearchResults model.query intent model.projectName )

        GotSearchResults results ->
            ( { model | results = Just results }, Cmd.none )

        FocusOn id ->
            ( model, Browser.Dom.focus id |> Task.attempt (\_ -> NoOp) )

        NoOp ->
            ( model, Cmd.none )



-- VIEW


searchInput =
    E.html
        (Html.input
            [ Html.Attributes.type_ "text"
            , Html.Attributes.autofocus True
            , Html.Attributes.id "search-input"
            , Html.Attributes.placeholder "go http.ListenAndServe"
            , Html.Attributes.style "font-size" "16px"
            , Html.Attributes.style "font-family" "JetBrains Mono, monospace"
            , Html.Attributes.style "padding" "0.5rem"
            , Html.Attributes.style "width" "100%"
            , Html.Attributes.style "margin-bottom" "2rem"
            , Html.Events.onInput OnSearchInput
            ]
            []
        )


searchResults : Maybe (Result Http.Error APISchema.SearchResults) -> E.Element msg
searchResults request =
    case request of
        Just response ->
            case response of
                Ok results ->
                    E.column [ E.width E.fill ]
                        (List.map
                            (\r ->
                                E.row
                                    [ E.width E.fill
                                    , E.paddingXY 0 8
                                    , Border.color (E.rgb255 210 210 210)
                                    , Border.widthEach { top = 0, left = 0, bottom = 1, right = 0 }
                                    ]
                                    [ E.column []
                                        [ E.link [ E.paddingEach { top = 0, right = 0, bottom = 4, left = 0 } ]
                                            { url = Url.Builder.absolute [ r.projectName, "-", r.language, "-", r.path ] [ Url.Builder.string "id" r.id ]
                                            , label = E.el [ Font.underline ] (E.text r.searchKey)
                                            }
                                        , E.el
                                            [ Font.color (E.rgb 0.6 0.6 0.6)
                                            , Font.size 14
                                            ]
                                            (E.text (Util.shortProjectName r.path))
                                        ]
                                    , E.el
                                        [ E.alignRight
                                        , Font.color (E.rgb 0.6 0.6 0.6)
                                        , Font.size 14
                                        ]
                                        (E.text (Util.shortProjectName r.projectName))
                                    ]
                            )
                            results
                        )

                Err err ->
                    E.text (httpErrorToString err)

        Nothing ->
            E.text "loading.."
