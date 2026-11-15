POST {{base_url}}/auth/login
Content-Type: application/json
{"username":"{{username}}","password":"{{password}}"}

HTTP 200
[Captures]
accessToken: jsonpath "$.accessToken"

POST {{base_url}}/products
Authorization: Bearer {{accessToken}}
Content-Type: application/json
{"name":"Pizza"}

HTTP 201
[Captures]
productId: jsonpath "$.id"

GET {{base_url}}/products/{{productId}}
Authorization: Bearer {{accessToken}}

HTTP 200

POST {{base_url}}/echo
Authorization: Bearer {{accessToken}}
Content-Type: application/json
{"productId":{{productId}}}

HTTP 200
