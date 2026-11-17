GET {{base_url}}/products

HTTP 200
[Asserts]
jsonpath "$.products[0].id" == 9007199254740992
jsonpath "$.products[1].id" == 9007199254740993
jsonpath "$.products[*].id" count == 2
[Captures]
product_ids: jsonpath "$.products[*].id"

POST {{base_url}}/echo
{{product_ids}}
HTTP 204
