GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.name" == "Pizza"
jsonpath "$.price" == 4500
jsonpath "$.active" == true
jsonpath "$.empty" == ""
jsonpath "$.off" == false
jsonpath "$.nil" == null
GET http://example.invalid/health
HTTP 200
