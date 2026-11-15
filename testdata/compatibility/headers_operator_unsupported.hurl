GET http://example.invalid/health
HTTP 200
[Asserts]
header "X-Test" != "other"
GET http://example.invalid/missing
HTTP 404
