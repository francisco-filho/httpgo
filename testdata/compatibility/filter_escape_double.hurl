GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v==\"a\\\"b/c\\\\d\")].id" == "selected"

GET http://example.invalid/health
HTTP 200
