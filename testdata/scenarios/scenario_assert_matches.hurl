GET {{base_url}}/products/1
HTTP 200
[Asserts]
jsonpath "$.name" matches "^Pizza"
