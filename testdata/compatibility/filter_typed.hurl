GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v==1)].id" == "number"
jsonpath "$[?(@.v==true)].id" == "boolean"
jsonpath "$[?(@.v==null)].id" == "null"
jsonpath "$[?(@.v=='1')].id" == "string"

GET http://example.invalid/health
HTTP 200
