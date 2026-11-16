GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X" not contains ""
GET http://example.invalid/health
HTTP 200
