GET http://example.invalid/health
HTTP 200
[Asserts]
header "X-Test" == "first"
GET http://example.invalid/missing
HTTP 404
