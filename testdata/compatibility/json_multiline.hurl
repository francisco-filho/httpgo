PUT http://example.invalid/health
X-Probe: multiline
{
 "nested": {"value": [true, null]},
 "text": "HTTP 404 POST [Asserts] # \""
}
HTTP 200
GET http://example.invalid/missing
HTTP 404
