GET http://example.invalid/health
HTTP 200
[Asserts]
header "Missing" == ""
GET http://example.invalid/missing
HTTP 404
