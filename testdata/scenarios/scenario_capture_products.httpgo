POST {{base_url}}/products
Content-Type: application/json
{"name":"Pizza"}

HTTP 201
[Captures]
productId: jsonpath "$.id"

GET {{base_url}}/products/{{productId}}

HTTP 200

POST {{base_url}}/echo
Content-Type: application/json
{"productId":{{productId}}}

HTTP 200
