POST http://example.invalid/health
"HTTP 200 # [Captures]"
HTTP 200
GET http://example.invalid/missing
HTTP 404
