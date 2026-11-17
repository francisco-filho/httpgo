GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v==18446744073709551617)].id" == "b"

GET http://example.invalid/health
HTTP 200
