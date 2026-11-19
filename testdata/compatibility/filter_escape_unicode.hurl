GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v=='\\uD83C\\uDF55')].id" == "selected"

GET http://example.invalid/health
HTTP 200
