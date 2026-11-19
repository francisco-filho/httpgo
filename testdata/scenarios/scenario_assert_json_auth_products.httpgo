POST {{base_url}}/auth/login
Content-Type: application/json
{"username":"{{username}}","password":"{{password}}"}
HTTP 200
[Captures]
accessToken: jsonpath "$.accessToken"

POST {{base_url}}/products
Authorization: Bearer {{accessToken}}
Content-Type: application/json
{"name":"Pizza","price":4500,"active":true}
HTTP 201
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.id" == 9007199254740993
[Captures]
productId: jsonpath "$.id"

GET {{base_url}}/products/{{productId}}
Authorization: Bearer {{accessToken}}
HTTP 200
[Asserts]
jsonpath "$.name" == "Pizza"
header "X-Request-Id" exists
jsonpath "$.price" == 4500
jsonpath "$.active" == true
jsonpath "$.id" == 9007199254740993

POST {{base_url}}/echo
Authorization: Bearer {{accessToken}}
Content-Type: application/json
{"productId":{{productId}}}
HTTP 200
[Asserts]
jsonpath "$.productId" == 9007199254740993
