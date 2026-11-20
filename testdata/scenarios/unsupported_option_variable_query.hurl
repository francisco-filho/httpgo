GET http://example.invalid/first
GET http://example.invalid/late
HTTP 200
[Asserts]
variable "name" == "private"
