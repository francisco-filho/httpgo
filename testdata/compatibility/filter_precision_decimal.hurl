GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v==1.0000000000000000000002)].id" == "b"

GET http://example.invalid/health
HTTP 200
