GET {{base_url}}/jobs/current
HTTP 200
[Captures]
job_id: jsonpath "$.selectedId"
[Asserts]
jsonpath "$.jobs[?(@.id=='{{job_id}}')].status" == "READY"

GET {{base_url}}/jobs/{{job_id}}/successor
HTTP 200
[Captures]
job_id: jsonpath "$.id"
[Asserts]
jsonpath "$.status" == "READY"

GET {{base_url}}/jobs/{{job_id}}
HTTP 200
