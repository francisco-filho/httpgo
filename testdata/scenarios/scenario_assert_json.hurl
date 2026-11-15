GET {{base_url}}/products/1

HTTP 200
[Asserts]
jsonpath "$.name" == "Pizza"
jsonpath "$.price" == 4500
jsonpath "$.active" == true
