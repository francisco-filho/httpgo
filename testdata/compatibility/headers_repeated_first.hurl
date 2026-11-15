GET http://example.invalid/health
HTTP 200
[Asserts]
header "x-multi" == "first,whole"
header "X-Multi" contains "st,wh"
GET http://example.invalid/missing
HTTP 404
