GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v==9007199254740993)].id" == "b"

GET http://example.invalid/health
HTTP 200
