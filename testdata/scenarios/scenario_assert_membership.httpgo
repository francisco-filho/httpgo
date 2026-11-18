GET {{base_url}}/validation
HTTP 200
[Asserts]
jsonpath "$.codes" contains "REQUIRED"
jsonpath "$.codes" not contains "OTHER_TENANT"
jsonpath "$.passwordHash" not exists
header "Set-Cookie" not contains "Domain"
