module Style exposing (..)

import Element as E
import Element.Font as Font
import Element.Region as Region


font =
    Font.family [ Font.typeface "JetBrains Mono", Font.monospace ]


fontSize =
    Font.size 16


layout =
    [ font, fontSize ]


h1 attrs child =
    E.paragraph (List.concat [ attrs, [ Region.heading 1, Font.size 32, Font.bold ] ]) [ child ]


h2 attrs child =
    E.paragraph (List.concat [ attrs, [ Region.heading 2, Font.size 24, Font.bold ] ]) [ child ]


h3 attrs child =
    E.paragraph (List.concat [ attrs, [ Region.heading 3, Font.size 20, Font.bold ] ]) [ child ]


h4 attrs child =
    E.paragraph (List.concat [ attrs, [ Region.heading 4, Font.size 16, Font.bold ] ]) [ child ]


paragraph attrs child =
    E.paragraph (List.concat [ attrs, [ Font.size 16, Font.family [ Font.typeface "Verdana, Geneva, sans-serif" ] ] ]) child
