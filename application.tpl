{
    "abc": "{{ .PerfectSecret }}",
    "xyz": "{{ index . "ECRMS/PROD/CONFIGURATION_DB_PASSWORD" }}",
    "foo": "SAMPLE-{{ .Foo }}"
}