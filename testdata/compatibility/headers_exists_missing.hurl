GET http://example.invalid/health
HTTP *
[Asserts]
header "Missing" exists
GET http://example.invalid/missing
HTTP 404
