module Style exposing (..)

import Element as E
import Element.Font as Font


font =
    Font.family [ Font.typeface "JetBrains Mono", Font.monospace ]


h2 attrs child =
    E.el (List.concat [ attrs, [ Font.size 24, Font.bold ] ]) child
