GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v==1e-99999999999999999999999)].id" == "tiny"

GET http://example.invalid/health
HTTP 200
