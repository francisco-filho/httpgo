GET http://example.invalid/health
HTTP 200
[Asserts]
header "X-Multi" == "second"
GET http://example.invalid/missing
HTTP 404
