GET http://example.invalid/health
HTTP *
[Asserts]
header "X-Literal" == "{{name}}"
GET http://example.invalid/missing
HTTP 404
