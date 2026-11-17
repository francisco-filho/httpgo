GET {{base_url}}/first
GET {{base_url}}/products
HTTP 200
[Asserts]
jsonpath "$[?(@.x.length() == 1)].id" exists
