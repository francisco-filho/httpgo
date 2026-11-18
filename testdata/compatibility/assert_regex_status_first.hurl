GET http://example.invalid/capture
HTTP 201
[Asserts]
regex "no-group" not contains "x"
GET http://example.invalid/health
HTTP 200
