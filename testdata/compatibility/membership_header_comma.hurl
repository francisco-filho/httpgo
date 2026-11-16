GET http://example.invalid/capture
HTTP 200
[Asserts]
header "x" not contains "first,whole"
GET http://example.invalid/health
HTTP 200
