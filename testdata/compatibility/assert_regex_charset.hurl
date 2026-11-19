GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "(x)" not contains "y"
GET http://example.invalid/health
HTTP 200
