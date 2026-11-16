GET http://example.invalid/capture
HTTP 200
[Captures]
x: header "Set-Cookie" regex "refresh=([^;]+)"
GET http://example.invalid/health
HTTP 200
