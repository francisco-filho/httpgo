GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "(a)?b" not contains ""
GET http://example.invalid/health
HTTP 200
