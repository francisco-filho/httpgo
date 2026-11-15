GET http://example.invalid/health
HTTP 200
GET http://example.invalid/missing
[Asserts]
header "X-Test" exists
