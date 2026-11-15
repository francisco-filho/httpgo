GET http://example.invalid/health
HTTP 200
[Asserts]
header "X-Multi" exists
GET http://example.invalid/missing
HTTP 404
