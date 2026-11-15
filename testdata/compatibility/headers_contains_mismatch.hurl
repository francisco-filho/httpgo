GET http://example.invalid/health
HTTP 200
[Asserts]
header "X-Test" contains "VALUE"
GET http://example.invalid/missing
HTTP 404
