GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v=='x)].id[?(@.a==1)')].id" == "selected"

GET http://example.invalid/health
HTTP 200
