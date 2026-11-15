GET http://example.invalid/health
HTTP 200
GET http://example.invalid/missing
HTTP *
[Asserts]
header "X-Test" ==
