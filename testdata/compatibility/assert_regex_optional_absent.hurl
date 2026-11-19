GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "(a)?b" exists
GET http://example.invalid/health
HTTP 200
