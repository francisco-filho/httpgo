GET http://example.invalid/capture
HTTP 200
[Asserts]
regex "(first)(second)" == "first"
GET http://example.invalid/health
HTTP 200
