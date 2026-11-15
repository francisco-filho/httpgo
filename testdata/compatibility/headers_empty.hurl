GET http://example.invalid/health
HTTP *
[Asserts]
header "X-Empty" exists
header "x-empty" == ""
header "X-Empty" contains ""
header "X-Value" contains ""
GET http://example.invalid/missing
HTTP 404
